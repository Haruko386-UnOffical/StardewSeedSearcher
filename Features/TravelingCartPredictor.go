package Features

import (
	"StardewSeedSearcher/Core"
	"StardewSeedSearcher/Data"
	"math/rand"
	"sort"
	"strconv"
	"sync"
)

type CartItem struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Price    int    `json:"price"`
}

type CartDayResult struct {
	Day   int        `json:"day"`
	Items []CartItem `json:"items"`
}

type CartCondition struct {
	StartYear      int    `json:"startYear"`
	StartSeason    int    `json:"startSeason"`
	StartDay       int    `json:"startDay"`
	EndYear        int    `json:"endYear"`
	EndSeason      int    `json:"endSeason"`
	EndDay         int    `json:"endDay"`
	ItemName       string `json:"itemName"`
	RequireQty5    bool   `json:"requireQty5"`
	MinOccurrences int    `json:"minOccurrences"` // 默认值在前端处理
}

func (c *CartCondition) AbsoluteStartDay() int {
	return Core.DateToAbsoluteDay(c.StartYear, c.StartSeason, c.StartDay)
}

func (c *CartCondition) AbsoluteEndDay() int {
	return Core.DateToAbsoluteDay(c.EndYear, c.EndSeason, c.EndDay)
}

type CartDayMatch struct {
	Year        int    `json:"year"`
	Season      int    `json:"season"`
	Day         int    `json:"day"`
	AbsoluteDay int    `json:"absoluteDay"` // 修正为 int，用于排序
	ItemName    string `json:"itemName"`
	Quantity    int    `json:"quantity"`
	Price       int    `json:"price"`
}

// OptimizedItem 内部预处理的高速物品结构
type OptimizedItem struct {
	Key        string
	Name       string
	Price      int
	IsEligible bool
}

// 全局缓存，保证只初始化一次
var (
	cachedOptimizedItems []OptimizedItem
	optimizedItemsOnce   sync.Once
)

func getOptimizedItems() []OptimizedItem {
	optimizedItemsOnce.Do(func() {
		keys := make([]string, 0, len(Data.Objects))
		for k := range Data.Objects {
			keys = append(keys, k)
		}
		// 必须按 ID 排序，保证每次遍历的确定性（与 C# 字典解析顺序一致）
		sort.Slice(keys, func(i, j int) bool {
			id1, _ := strconv.Atoi(keys[i])
			id2, _ := strconv.Atoi(keys[j])
			return id1 < id2
		})

		cachedOptimizedItems = make([]OptimizedItem, 0, len(keys))
		for _, k := range keys {
			item := Data.Objects[k]
			isEligible := true

			itemID, err := strconv.Atoi(item.Id)
			if err != nil || itemID < 2 || itemID > 789 || item.Price <= 0 ||
				item.OffLimits || item.Category >= 0 || item.Category == -999 ||
				item.Type == "Arch" || item.Type == "Minerals" || item.Type == "Quest" {
				isEligible = false
			}

			cachedOptimizedItems = append(cachedOptimizedItems, OptimizedItem{
				Key:        k,
				Name:       item.Name,
				Price:      item.Price,
				IsEligible: isEligible,
			})
		}
	})
	return cachedOptimizedItems
}

func isSkillBook(name string) bool {
	for _, book := range Data.SkillBooks {
		if book == name {
			return true
		}
	}
	return false
}

type TravelingCartPredictor struct {
	IsEnabled  bool            `json:"isEnabled"`
	Conditions []CartCondition `json:"conditions"`
	Name       string          `json:"name"`
}

func NewTravelingCartPredictor() *TravelingCartPredictor {
	return &TravelingCartPredictor{
		Name:       "猪车预测",
		IsEnabled:  false,
		Conditions: []CartCondition{},
	}
}

func (t *TravelingCartPredictor) GetName() string {
	return t.Name
}

func (t *TravelingCartPredictor) GetIsEnabled() bool {
	return t.IsEnabled
}

func (t *TravelingCartPredictor) SetIsEnabled(enabled bool) {
	t.IsEnabled = enabled
}

func (t *TravelingCartPredictor) Check(seed int32, useLegacyRandom bool) bool {
	if len(t.Conditions) == 0 {
		return true
	}

	// 1. 动态排序优化 (按照计算代价从小到大排序，优先执行最稀有、最易排除的条件)
	// 在 Go 中，每次 Check 排序开销可能略大，但相比节约的 RNG 次数是值得的
	sortedConditions := make([]CartCondition, len(t.Conditions))
	copy(sortedConditions, t.Conditions)
	sort.Slice(sortedConditions, func(i, j int) bool {
		return t.EstimateCostPerCondition(sortedConditions[i]) < t.EstimateCostPerCondition(sortedConditions[j])
	})

	guaranteeSeed := Core.GetRandomSeed(12*seed, 0, 0, 0, 0, useLegacyRandom)
	originalGuarantee := rand.New(rand.NewSource(int64(guaranteeSeed))).Intn(29) + 2 // Next(2, 31)

	// 所有条件都必须满足（AND）
	for _, condition := range sortedConditions {
		matches := 0
		minOccurrences := condition.MinOccurrences
		if minOccurrences < 1 {
			minOccurrences = 1
		}

		for day := condition.AbsoluteStartDay(); day <= condition.AbsoluteEndDay(); day++ {
			if !t.IsCartDay(day) {
				continue
			}

			// 调用高性能匹配
			if t.InternalDayMatch(seed, day, originalGuarantee, condition, useLegacyRandom) {
				matches++
				if matches >= minOccurrences {
					break
				}
			}
		}

		if matches < minOccurrences {
			return false
		}
	}

	return true
}

func (t *TravelingCartPredictor) EstimateCostPerCondition(c CartCondition) float64 {
	maxCalls := 1381.0
	calls := 730.0

	if isSkillBook(c.ItemName) {
		calls = 0.05 * maxCalls
	}

	totalDays := 0
	for day := c.AbsoluteStartDay(); day <= c.AbsoluteEndDay(); day++ {
		if t.IsCartDay(day) {
			totalDays++
		}
	}

	return float64(totalDays) * calls
}

func (t *TravelingCartPredictor) EstimateCost(useLegacyRandom bool) int {
	if len(t.Conditions) == 0 {
		return 0
	}
	bestCost := t.EstimateCostPerCondition(t.Conditions[0])
	for _, c := range t.Conditions {
		cost := t.EstimateCostPerCondition(c)
		if cost < bestCost {
			bestCost = cost
		}
	}
	return int(bestCost)
}

func (t *TravelingCartPredictor) GetConfigDescription() string {
	return strconv.Itoa(len(t.Conditions)) + " 个条件"
}

func (t *TravelingCartPredictor) IsCartDay(day int) bool {
	dayOfWeek := day % 7
	dayOfYear := day % 112

	if dayOfWeek == 5 || dayOfWeek == 0 {
		return true
	}
	if dayOfYear >= 15 && dayOfYear <= 17 { // 沙漠节
		return true
	}
	if dayOfYear >= 99 && dayOfYear <= 101 { // 夜市
		return true
	}
	return false
}

func (t *TravelingCartPredictor) GetCartMatches(seed int32, useLegacyRandom bool) []CartDayMatch {
	cartMatches := make([]CartDayMatch, 0)

	for _, condition := range t.Conditions {
		matches := t.FindAllMatches(seed, condition.AbsoluteStartDay(), condition.AbsoluteEndDay(),
			condition.ItemName, condition.RequireQty5, useLegacyRandom, 2147483647)
		cartMatches = append(cartMatches, matches...)
	}

	return cartMatches
}

func (t *TravelingCartPredictor) FindAllMatches(seed int32, startDay, endDay int, itemName string, requireQty5, useLegacyRandom bool, stopAt int) []CartDayMatch {
	matches := make([]CartDayMatch, 0)

	guaranteeSeed := Core.GetRandomSeed(12*seed, 0, 0, 0, 0, useLegacyRandom)
	originalGuarantee := rand.New(rand.NewSource(int64(guaranteeSeed))).Intn(29) + 2

	for day := startDay; day <= endDay; day++ {
		if !t.IsCartDay(day) {
			continue
		}

		result := t.PredictCartDay(seed, day, originalGuarantee, useLegacyRandom)

		for _, item := range result.Items {
			if item.Name != itemName {
				continue
			}
			if requireQty5 && item.Quantity != 5 {
				continue
			}

			year, season, dayOfMonth := Core.AbsoluteDayToDate(day)

			matches = append(matches, CartDayMatch{
				Year:        year,
				Season:      season,
				Day:         dayOfMonth,
				AbsoluteDay: day,
				ItemName:    item.Name,
				Quantity:    item.Quantity,
				Price:       item.Price,
			})
			break
		}

		if len(matches) >= stopAt {
			break
		}
	}
	return matches
}

// InternalDayMatch 高性能、提前熔断匹配逻辑
func (t *TravelingCartPredictor) InternalDayMatch(seed int32, day, originalGuarantee int, cond CartCondition, useLegacyRandom bool) bool {
	isBookSearch := isSkillBook(cond.ItemName)

	if isBookSearch {
		skillHash := Core.GetHashFromString("travelerSkillBook")
		skillSeed := Core.GetRandomSeed(skillHash, seed, int32(day), 0, 0, useLegacyRandom)
		if rand.New(rand.NewSource(int64(skillSeed))).Float64() >= 0.05 {
			return false
		}
	}

	mainSeed := Core.GetRandomSeed(int32(day), seed/2, 0, 0, 0, useLegacyRandom)
	rng := rand.New(rand.NewSource(int64(mainSeed)))

	allItems := getOptimizedItems()

	// 使用栈内存数组模拟 C# stackalloc 的手动插入排序，避免全量排序和内存分配
	topKeys := [10]int32{2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647}
	topIndices := [10]int{}

	for i := 0; i < len(allItems); i++ {
		randomKey := rng.Int31()
		if !allItems[i].IsEligible {
			continue
		}

		if randomKey < topKeys[9] {
			j := 8
			for j >= 0 && topKeys[j] > randomKey {
				topKeys[j+1] = topKeys[j]
				topIndices[j+1] = topIndices[j]
				j--
			}
			topKeys[j+1] = randomKey
			topIndices[j+1] = i
		}
	}

	seenRareSeed := false

	if !isBookSearch {
		for i := 0; i < 10; i++ {
			item := allItems[topIndices[i]]
			if item.Name == "Rare Seed" {
				seenRareSeed = true
			}

			if item.Name == cond.ItemName {
				for k := 0; k < i; k++ {
					rng.Intn(10)  // Next(1, 11)
					rng.Intn(3)   // Next(3, 6)
					rng.Float64() // NextDouble()
				}

				rng.Intn(10)
				rng.Intn(3)

				qty := 1
				if rng.Float64() < 0.1 {
					qty = 5
				}

				if !cond.RequireQty5 || qty == 5 {
					return true
				}
				return false
			}
		}
		return false
	}

	// 如果搜的是书，只有前置概率命中了才会走到这里，必须把后续的序列吃满
	for i := 0; i < 10; i++ {
		if allItems[topIndices[i]].Name == "Rare Seed" {
			seenRareSeed = true
		}
		rng.Intn(10)
		rng.Intn(3)
		rng.Float64()
	}

	if t.calculateVisitsRemaining(day, originalGuarantee) == 0 {
		rng.Intn(10)
		rng.Intn(3)
		rng.Float64()
	}

	for i := 0; i < 645; i++ {
		rng.Int31() // 645 次家具消耗
	}

	rng.Intn(10)

	if (day-1)/28 < 2 && !seenRareSeed {
		rng.Float64()
	}

	bookName := Data.SkillBooks[rng.Intn(len(Data.SkillBooks))]
	return bookName == cond.ItemName
}

// PredictCartDay 完整预测当日所有物品（用于最终结果输出展示）
func (t *TravelingCartPredictor) PredictCartDay(gameID int32, day, originalGuarantee int, useLegacyRandom bool) CartDayResult {
	result := CartDayResult{Day: day, Items: []CartItem{}}

	seed := Core.GetRandomSeed(int32(day), gameID/2, 0, 0, 0, useLegacyRandom)
	rng := rand.New(rand.NewSource(int64(seed)))

	selectedItemIndices := t.getRandomItemIndices(rng)
	seenRareSeed := false
	allItems := getOptimizedItems()

	for i := 0; i < len(selectedItemIndices); i++ {
		item := allItems[selectedItemIndices[i]]

		priceBase := (rng.Intn(10) + 1) * 100
		priceMult := (rng.Intn(3) + 3) * item.Price
		price := priceBase
		if priceMult > priceBase {
			price = priceMult
		}

		qty := 1
		if rng.Float64() < 0.1 {
			qty = 5
		}

		if item.Name == "Rare Seed" {
			seenRareSeed = true
		}

		result.Items = append(result.Items, CartItem{
			Category: "基础物品" + strconv.Itoa(i+1),
			Name:     item.Name,
			Quantity: qty,
			Price:    price,
		})
	}

	visitsNow := t.calculateVisitsRemaining(day, originalGuarantee)
	if visitsNow == 0 {
		rng.Intn(10)
		rng.Intn(3)
		rng.Float64()
	}

	for i := 0; i < 645; i++ {
		rng.Int31()
	}
	rng.Intn(10)

	season := (day - 1) / 28
	if season < 2 && !seenRareSeed {
		rng.Float64()
	}

	skillHash := Core.GetHashFromString("travelerSkillBook")
	skillSeed := Core.GetRandomSeed(skillHash, gameID, int32(day), 0, 0, useLegacyRandom)
	rngSkill := rand.New(rand.NewSource(int64(skillSeed)))

	if rngSkill.Float64() < 0.05 {
		bookName := Data.SkillBooks[rng.Intn(len(Data.SkillBooks))]
		result.Items = append(result.Items, CartItem{
			Category: "技能书",
			Name:     bookName,
			Quantity: -1,
			Price:    6000,
		})
	} else {
		result.Items = append(result.Items, CartItem{
			Category: "技能书",
			Name:     "(None)",
			Quantity: 0,
			Price:    0,
		})
	}

	return result
}

// getRandomItemIndices 获取随机物品的索引列表
func (t *TravelingCartPredictor) getRandomItemIndices(rng *rand.Rand) []int {
	allItems := getOptimizedItems()

	topKeys := [10]int32{2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647, 2147483647}
	topIndices := [10]int{}

	for i := 0; i < len(allItems); i++ {
		randomKey := rng.Int31()
		if !allItems[i].IsEligible {
			continue
		}

		if randomKey < topKeys[9] {
			j := 8
			for j >= 0 && topKeys[j] > randomKey {
				topKeys[j+1] = topKeys[j]
				topIndices[j+1] = topIndices[j]
				j--
			}
			topKeys[j+1] = randomKey
			topIndices[j+1] = i
		}
	}

	result := make([]int, 10)
	for i := 0; i < 10; i++ {
		result[i] = topIndices[i]
	}
	return result
}

func (t *TravelingCartPredictor) calculateVisitsRemaining(day, originalGuarantee int) int {
	visitsNow := originalGuarantee - (day / 7) - ((day + 2) / 7)
	if day >= 99 {
		visitsNow--
	}
	if day >= 100 {
		visitsNow--
	}
	if day >= 101 {
		visitsNow--
	}
	return visitsNow
}
