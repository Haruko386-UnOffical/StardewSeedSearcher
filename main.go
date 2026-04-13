package main

import (
	"StardewSeedSearcher/Data"
	"StardewSeedSearcher/Features"
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"math"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// 全局 WebSocket 连接管理
var (
	activeConnections sync.Map // 并发安全的 Map，存储活跃的 WS 连接
	upgrader          = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }, // 允许跨域
	}
)

// 全局搜索控制
var (
	currentSearchCancel context.CancelFunc
	searchMutex         sync.Mutex // 保护 currentSearchCancel 的并发写入
)

func main() {
	// 初始化数据
	Data.Initialize()

	// 设置 Gin 模式，隐藏 Debug 日志保持控制台清爽
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 配置 CORS，允许前端本地访问
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Length", "Content-Type"},
	}))

	// 注册路由
	r.GET("/ws", wsHandler)
	r.GET("/api/cart-items", getCartItems)
	r.GET("/api/seasons", getSeasons)
	r.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "1.0"}) })
	r.POST("/api/stop", stopSearch)
	r.POST("/api/search", apiSearch)

	log.Println("╔════════════════════════════════════════╗")
	log.Println("║  🌾 星露谷种子搜索器 - Go Web 服务启动  ║")
	log.Println("╚════════════════════════════════════════╝")
	log.Println("✓ 服务器地址: http://localhost:5000")
	log.Println("📝 请打开 index.html 开始使用")

	// 启动服务
	if err := r.Run(":5000"); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}

// ---------------- 核心并发区域 (留给你发挥) ----------------

func apiSearch(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. 初始化启用的预测器功能
	features := InitializeFeatures(req)
	totalSeeds := int(req.EndSeed) - req.StartSeed + 1

	// 2. 取消之前的搜索，并创建新的上下文 (Context)
	searchMutex.Lock()
	if currentSearchCancel != nil {
		currentSearchCancel()
	}
	// userCtx 用于处理"用户主动点击停止"
	userCtx, cancelUser := context.WithCancel(context.Background())
	currentSearchCancel = cancelUser
	searchMutex.Unlock()

	// limitCtx 用于处理"搜够了数量自动停止"
	limitCtx, cancelLimit := context.WithCancel(userCtx)

	// 发送开始消息给前端
	_ = BroadcastMessage(SearchMessage{Type: "start", Data: gin.H{"type": "start", "total": totalSeeds}})
	c.JSON(http.StatusOK, gin.H{"message": "Search started."})

	msgChan := make(chan SearchMessage, 1000000) // 有缓存通道
	var wg sync.WaitGroup

	// 全局原子计数器，保证在多个 Worker 并发累加时数据不冲突
	var globalChecked int64
	var foundCount int32
	startTime := time.Now()

	// 1. 启动【消费者】Goroutine (单一写入器)
	// 踩坑预警：Gorilla WebSocket 的 WriteJSON 是非线程安全的，绝对不能在多个 Worker 里直接广播。
	// 这里我们用单独一个 Goroutine 专门负责读取 Channel 并发送消息，完美避开并发写入崩溃的问题。
	go func() {
		for msg := range msgChan {
			_ = BroadcastMessage(msg)
		}
	}()

	// 2. 分配任务给【生产者】Worker Goroutines
	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localChecked := 0
			// 步长分配 (Stride): 每个 Worker 跳跃式检查种子，自动实现绝对的负载均衡
			// 例如 4个Worker：Worker0 查 0,4,8... Worker1 查 1,5,9...

			for seed := int64(req.StartSeed) + int64(workerID); seed <= int64(req.EndSeed); seed += int64(numWorkers) {
				// 每次循环先检查是否收到了取消信号 (如果收到了，直接 return 结束当前 Worker)
				select {
				case <-limitCtx.Done():
					return
				default:
				}

				// 核心运算：检查种子
				match := checkSeed(int32(seed), req.UseLegacyRandom, features)

				currentChecked := atomic.AddInt64(&globalChecked, 1)
				localChecked++
				// 找到符合条件的种子
				if match {
					currentFound := atomic.AddInt32(&foundCount, 1)
					if currentFound <= int32(req.OutputLimit) {
						details := CollectAllDetails(int32(seed), req.UseLegacyRandom, features)
						// 将结果推入通道
						msgChan <- SearchMessage{Type: "found",
							Data: gin.H{
								"type":            "found",
								"seed":            seed,
								"details":         details,
								"enabledFeatures": GetEnabledFeatures(req),
							},
						}

						// 如果达到前端要求的上限，触发 cancelLimit，其他所有 Worker 收到信号后会立即停止
						if currentFound == int32(req.OutputLimit) {
							cancelLimit()
							return
						}
					} else {
						cancelLimit()
						return
					}

				}
				// 每隔1000次检查，发送一次进度更新
				if localChecked%1000 == 0 {
					elapsed := time.Since(startTime).Seconds()
					elapsed = math.Max(elapsed, 0.001) // 防止除以 0

					// 这里使用了 select 的非阻塞发送技巧：
					// 如果 msgChan 满了，它会直接走 default 丢弃这次进度播报，而不是让 Worker 停下来干等，极大提升性能
					select {
					case msgChan <- SearchMessage{
						Type: "search",
						Data: gin.H{
							"type":         "progress",
							"checkedCount": currentChecked,
							"total":        totalSeeds,
							"progress":     (float64(currentChecked) / float64(totalSeeds)) * 100,
							"speed":        float64(currentChecked) / elapsed,
							"elapsed":      elapsed,
						},
					}:
					default:
					}
				}
			}
		}(i)
	}

	// 3. 启动【监控】Goroutine (收尾工作)
	// 它专门负责在一旁等待，等所有人都下班了，负责关灯（关闭 Channel）
	go func() {
		defer cancelLimit() // 确保退出时释放资源
		wg.Wait()           // 阻塞直到所有 wg.Done() 被调用

		elapsed := time.Since(startTime).Seconds()
		elapsed = math.Max(elapsed, 0.001) // ✅ 修正：必须用 Max，防止分母为 0
		finalChecked := atomic.LoadInt64(&globalChecked)
		finalFound := atomic.LoadInt32(&foundCount)

		// 确保最后一次进度精确达到实际检查量
		msgChan <- SearchMessage{
			Type: "progress",
			Data: gin.H{
				"type":         "progress",
				"checkedCount": finalChecked,
				"progress":     (float64(finalChecked) / float64(totalSeeds)) * 100,
				"speed":        float64(finalChecked) / elapsed,
				"elapsed":      elapsed,
			},
		}

		// 发送消息完成
		msgChan <- SearchMessage{
			Type: "complete",
			Data: gin.H{
				"type":       "complete",
				"totalFound": finalFound,
				"elapsed":    elapsed,
				"cancelled":  userCtx.Err() != nil, // 如果 userCtx 被 cancel，说明是用户主动点击的停止
			},
		}

		// ✅ 正确做法：必须在后台等待彻底完成后，由最后这个“监工”负责关通道
		close(msgChan)
	}()

	// 主函数直接结束，不需要在这里写 close，让监控 Goroutine 去处理收尾。
}

// checkSeed 是单个种子的检查逻辑，你在 Worker 里调用它
func checkSeed(seed int32, useLegacy bool, features []Features.ISearchFeature) bool {
	for _, f := range features {
		if !f.Check(seed, useLegacy) {
			return false
		}
	}
	return true
}

// ---------------- API 和基础工具 ----------------

func wsHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	connID := c.ClientIP() + ":" + c.Request.RemoteAddr
	activeConnections.Store(connID, ws)

	defer func() {
		activeConnections.Delete(connID)
		ws.Close()
	}()

	// 保持连接并丢弃客户端发来的消息
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
}

func BroadcastMessage(msg SearchMessage) error {
	var err error
	activeConnections.Range(func(key, value interface{}) bool {
		ws := value.(*websocket.Conn)
		if writeErr := ws.WriteJSON(msg.Data); writeErr != nil {
			err = writeErr
			ws.Close()
			activeConnections.Delete(key)
		}
		return true // 继续遍历
	})
	return err
}

func getCartItems(c *gin.Context) {
	itemsMap := make(map[string]bool)
	for _, item := range Data.Objects {
		if !item.OffLimits && item.Price > 0 {
			itemsMap[item.Name] = true
		}
	}
	for _, book := range Data.SkillBooks {
		itemsMap[book] = true
	}
	var items []string
	for k := range itemsMap {
		items = append(items, k)
	}
	c.JSON(http.StatusOK, items)
}

func getSeasons(c *gin.Context) {
	seasons := []gin.H{
		{"id": 0, "name": "春"},
		{"id": 1, "name": "夏"},
		{"id": 2, "name": "秋"},
		{"id": 3, "name": "冬"},
	}
	c.JSON(http.StatusOK, seasons)
}

func stopSearch(c *gin.Context) {
	searchMutex.Lock()
	if currentSearchCancel != nil {
		currentSearchCancel()
	}
	searchMutex.Unlock()
	c.JSON(http.StatusOK, gin.H{"message": "Stop requested."})
}

// ---------------- 数据结构与前端映射 ----------------

type SearchMessage struct {
	Type string
	Data interface{}
}

// 与 frontend JSON 对应的数据结构
type SearchRequest struct {
	StartSeed               int                               `json:"startSeed"`
	EndSeed                 int64                             `json:"endSeed"`
	UseLegacyRandom         bool                              `json:"useLegacyRandom"`
	OutputLimit             int                               `json:"outputLimit"`
	WeatherConditions       []Features.WeatherCondition       `json:"weatherConditions"`
	FairyConditions         []Features.FairyCondition         `json:"fairyConditions"`
	MineChestConditions     []Features.MineChestCondition     `json:"mineChestConditions"`
	MonsterLevelConditions  []Features.MonsterLevelCondition  `json:"monsterLevelConditions"`
	DesertFestivalCondition *Features.DesertFestivalPredictor `json:"desertFestivalCondition"`
	CartConditions          []Features.CartCondition          `json:"cartConditions"`
}

func InitializeFeatures(req SearchRequest) []Features.ISearchFeature {
	var features []Features.ISearchFeature

	if len(req.WeatherConditions) > 0 {
		p := Features.NewWeatherPredictor()
		p.IsEnabled = true
		p.Conditions = req.WeatherConditions
		features = append(features, p)
	}
	if len(req.FairyConditions) > 0 {
		p := Features.NewFairyPredictor()
		p.IsEnabled = true
		p.Conditions = req.FairyConditions
		features = append(features, p)
	}
	if len(req.MineChestConditions) > 0 {
		p := Features.NewMineChestPredictor(req.MineChestConditions)
		p.IsEnabled = true
		p.Conditions = req.MineChestConditions
		features = append(features, p)
	}
	if len(req.MonsterLevelConditions) > 0 {
		p := Features.NewMonsterLevelPredictor()
		p.IsEnabled = true
		p.Conditions = req.MonsterLevelConditions
		features = append(features, p)
	}
	if req.DesertFestivalCondition != nil && (req.DesertFestivalCondition.RequireJas || req.DesertFestivalCondition.RequireLeah) {
		req.DesertFestivalCondition.IsEnabled = true
		req.DesertFestivalCondition.Name = "沙漠节"
		features = append(features, req.DesertFestivalCondition)
	}
	if len(req.CartConditions) > 0 {
		p := Features.NewTravelingCartPredictor()
		p.IsEnabled = true
		p.Conditions = req.CartConditions
		features = append(features, p)
	}
	return features
}

func CollectAllDetails(seed int32, useLegacy bool, features []Features.ISearchFeature) gin.H {
	details := gin.H{}

	for _, f := range features {
		switch p := f.(type) {
		case *Features.WeatherPredictor:
			weatherMap, greenRainDay := p.PredictWeatherWithDetail(seed, useLegacy)
			details["weather"] = Features.ExtractWeatherDetail(weatherMap, greenRainDay)
		case *Features.FairyPredictor:
			details["fairy"] = gin.H{"days": p.GetFairyDays(int(seed), useLegacy)}
		case *Features.MineChestPredictor:
			details["mineChest"] = p.GetDetails(seed, useLegacy)
		case *Features.MonsterLevelPredictor:
			details["monsterLevel"] = p.GetDetails(int(seed), useLegacy)
		case *Features.DesertFestivalPredictor:
			details["desertFestival"] = p.GetVendorDetail(int(seed), useLegacy)
		case *Features.TravelingCartPredictor:
			details["cart"] = gin.H{"matches": p.GetCartMatches(seed, useLegacy)}
		}
	}
	return details
}

func GetEnabledFeatures(req SearchRequest) gin.H {
	return gin.H{
		"weather":        len(req.WeatherConditions) > 0,
		"fairy":          len(req.FairyConditions) > 0,
		"mineChest":      len(req.MineChestConditions) > 0,
		"monsterLevel":   len(req.MonsterLevelConditions) > 0,
		"desertFestival": req.DesertFestivalCondition != nil && (req.DesertFestivalCondition.RequireJas || req.DesertFestivalCondition.RequireLeah),
		"cart":           len(req.CartConditions) > 0,
	}
}
