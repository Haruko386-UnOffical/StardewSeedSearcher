package Features

import (
	"StardewSeedSearcher/Core"
	"math/rand"
	"strconv"
	"strings"
)

// Season 季节枚举
type Season int

const (
	Spring Season = iota
	Summer
	Fall
)

// String 返回季节中文名
func (s Season) String() string {
	switch s {
	case Spring:
		return "春"
	case Summer:
		return "夏"
	case Fall:
		return "秋"
	default:
		return "?"
	}
}

// WeatherCondition 天气筛选条件
type WeatherCondition struct {
	Season      Season `json:"season"`
	StartDay    int    `json:"startDay"`
	EndDay      int    `json:"endDay"`
	MinRainDays int    `json:"minRainDays"`
}

// String 格式化输出雨天
func (w WeatherCondition) String() string {
	seasonName := w.Season.String()
	return seasonName + strconv.Itoa(w.StartDay) + "-" +
		seasonName + strconv.Itoa(w.EndDay) + ": 最少" +
		strconv.Itoa(w.MinRainDays) + "个雨天"
}

// AbsoluteStartDay 计算绝对开始天数
func (w WeatherCondition) AbsoluteStartDay() int {
	return int(w.Season)*28 + w.StartDay
}

// AbsoluteEndDay 计算绝对结束天数
func (w WeatherCondition) AbsoluteEndDay() int {
	return int(w.Season)*28 + w.EndDay
}

// WeatherPredictor 天气预测功能
type WeatherPredictor struct {
	Conditions   []WeatherCondition `json:"conditions"`
	Name         string             `json:"name"`
	IsEnabled    bool               `json:"isEnabled"`
	locationHash int32
}

// NewWeatherPredictor 创建新的天气预测器
func NewWeatherPredictor() *WeatherPredictor {
	return &WeatherPredictor{
		Conditions:   []WeatherCondition{},
		Name:         "天气预测",
		IsEnabled:    false,
		locationHash: Core.GetHashFromString("location_weather"),
	}
}

// Check 检查种子是否符合筛选条件
func (w *WeatherPredictor) Check(gameID int32, useLegacyRandom bool) bool {
	if len(w.Conditions) == 0 {
		return true
	}
	greenRainDay := w.GetGreenRainDay(gameID, useLegacyRandom)

	for _, condition := range w.Conditions {
		rainCount := 0

		// 检查每一天
		for day := condition.AbsoluteStartDay(); day <= condition.AbsoluteEndDay(); day++ {
			season := int(condition.Season)
			dayOfMonth := ((day - 1) % 28) + 1

			// 检查是否下雨
			if w.isRainDay(season, dayOfMonth, day, gameID, useLegacyRandom, greenRainDay) {
				rainCount++
				// 剪枝 已经满足最少雨天数，不用算后面的
				if rainCount >= condition.MinRainDays {
					break
				}
			}
		}

		if rainCount < condition.MinRainDays {
			return false
		}
	}
	return true
}

// GetGreenRainDay 计算绿雨日期
func (w *WeatherPredictor) GetGreenRainDay(gameID int32, useLegacyRandom bool) int {
	greenRainSeed := Core.GetRandomSeed(777, gameID, 0, 0, 0, useLegacyRandom)
	greenRainRng := rand.New(rand.NewSource(int64(greenRainSeed)))
	greenRainDays := []int{5, 6, 7, 14, 15, 16, 18, 23}
	return greenRainDays[greenRainRng.Intn(len(greenRainDays))]
}

// isRainDaySpringFall 按概率计算春秋雨天
func (w *WeatherPredictor) isRainDaySpringFall(absoluteDay int, gameID int32, useLegacyRandom bool) bool {
	seed := Core.GetRandomSeed(w.locationHash, gameID, int32(absoluteDay-1), 0, 0, useLegacyRandom)
	rng := rand.New(rand.NewSource(int64(seed)))
	// 春季和秋季的普通日期：18.3% 概率
	return rng.Float64() < 0.183
}

// isRainDay 判断是否下雨
func (w *WeatherPredictor) isRainDay(season, dayOfMonth, absoluteDay int, gameID int32, useLegacyRandom bool, greenRainDay int) bool {
	// 季节第一天强制晴天
	if dayOfMonth == 1 {
		return false
	}

	if season == 0 {
		if dayOfMonth == 2 || dayOfMonth == 4 || dayOfMonth == 5 {
			return false
		} // 晴天
		if dayOfMonth == 3 {
			return true
		} // 雨天
		if dayOfMonth == 13 || dayOfMonth == 24 {
			return false
		} // 节日晴天
		return w.isRainDaySpringFall(int(gameID), int32(absoluteDay), useLegacyRandom)
	}
	if season == 1 {
		if dayOfMonth == greenRainDay {
			return true
		} // 绿雨
		if dayOfMonth == 11 || dayOfMonth == 28 {
			return false
		} // 节日
		if dayOfMonth%13 == 0 {
			return true
		} // 雷暴天

		// 普通雨天：概率随日期递增
		rainSeed := Core.GetRandomSeed(int32(absoluteDay-1), gameID/2, Core.GetHashFromString("summer_rain_chance"), 0, 0, useLegacyRandom)
		rainRng := rand.New(rand.NewSource(int64(rainSeed)))
		rainChance := 0.12 + 0.003*float64(dayOfMonth-1)
		return rainRng.Float64() < rainChance
	}

	// 秋季 (season 2)
	// 目前只支持第一年春夏秋
	if dayOfMonth == 16 || dayOfMonth == 27 {
		return false // 节日固定晴天
	}
	return w.isRainDaySpringFall(int(gameID), int32(absoluteDay), useLegacyRandom)
}

func (w *WeatherPredictor) EstimateCost(useLegacyRandom bool) int {
	if len(w.Conditions) == 0 {
		return 0
	}

	totalDays := 0
	for _, condition := range w.Conditions {
		totalDays += condition.AbsoluteEndDay() - condition.AbsoluteStartDay() + 1
	}
	return totalDays + 56
}

func (w *WeatherPredictor) GetConfigDescription() string {
	if len(w.Conditions) == 0 {
		return "无筛选条件"
	}

	description := make([]string, len(w.Conditions))
	for i, c := range w.Conditions {
		description[i] = c.String()
	}
	return strings.Join(description, ";")
}

// GetName 实现 ISearchFeature 接口
func (w *WeatherPredictor) GetName() string {
	return w.Name
}

// GetIsEnabled 实现 ISearchFeature 接口
func (w *WeatherPredictor) GetIsEnabled() bool {
	return w.IsEnabled
}

// PredictWeatherWithDetail 预测天气并返回详细信息（用于前端展示）
func (w *WeatherPredictor) PredictWeatherWithDetail(gameID int32, useLegacyRandom bool) (map[int]bool, int) {
	weather := make(map[int]bool)

	greenRainDay := w.GetGreenRainDay(gameID, useLegacyRandom)

	for absoluteDay := 1; absoluteDay <= 84; absoluteDay++ {
		season := (absoluteDay - 1) / 28
		dayOfMonth := ((absoluteDay - 1) % 28) + 1

		isRain := w.isRainDay(season, dayOfMonth, absoluteDay, gameID, useLegacyRandom, greenRainDay)
		weather[absoluteDay] = isRain
	}
	return weather, greenRainDay
}

// WeatherDetailResult 天气详情结果
type WeatherDetailResult struct {
	SpringRain   []int `json:"springRain"`
	SummerRain   []int `json:"summerRain"`
	FallRain     []int `json:"fallRain"`
	GreenRainDay int   `json:"greenRainDay"`
}

// ExtractWeatherDetail 从天气字典提取雨天列表和详细信息
func ExtractWeatherDetail(weather map[int]bool, greenRainDay int) *WeatherDetailResult {
	result := &WeatherDetailResult{
		SpringRain:   []int{},
		SummerRain:   []int{},
		FallRain:     []int{},
		GreenRainDay: greenRainDay,
	}

	for day := 1; day <= 28; day++ {
		if weather[day] {
			result.SpringRain = append(result.SpringRain, day)
		}
		if weather[day+28] {
			result.SummerRain = append(result.SummerRain, day)
		}
		if weather[day+56] {
			result.FallRain = append(result.FallRain, day)
		}
	}

	return result
}
