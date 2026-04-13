package Features

import (
	"StardewSeedSearcher/Core"
	"math/rand"
)

type FairyCondition struct {
	StartYear   int `json:"startYear"`
	StartSeason int `json:"startSeason"`
	StartDay    int `json:"startDay"`
	EndYear     int `json:"endYear"`
	EndSeason   int `json:"endSeason"`
	EndDay      int `json:"endDay"`
}

type FairyPredictor struct {
	Conditions []FairyCondition `json:"conditions"`
	Name       string           `json:"name"`
	IsEnabled  bool             `json:"isEnabled"`
}

func (f *FairyPredictor) GetName() string {
	return f.Name
}

func (f *FairyPredictor) GetIsEnabled() bool {
	return f.IsEnabled
}

func (f *FairyPredictor) Check(gameID int32, useLegacyRandom bool) bool {
	if len(f.Conditions) == 0 {
		return true
	}

	for _, condition := range f.Conditions {
		startAbs := Core.DateToAbsoluteDay(condition.StartYear, condition.StartSeason, condition.StartDay)
		endAbs := Core.DateToAbsoluteDay(condition.EndYear, condition.EndSeason, condition.EndDay)

		foundInRange := false

		for d := startAbs; d <= endAbs; d++ {
			_, season, _ := Core.AbsoluteDayToDate(d)
			if season >= 3 {
				continue
			} // 跳过冬天
			if f.HasFairy(gameID, int32(d), useLegacyRandom) {
				foundInRange = true
				break
			}
		}
		if !foundInRange {
			return false
		}
	}
	return true
}

// EstimateCost 估算搜索成本
func (f *FairyPredictor) EstimateCost(useLegacyRandom bool) int {
	if len(f.Conditions) == 0 {
		return 0
	}

	// 旧随机:1次随机判断
	// 新随机:10次跳过 + 1次判断 = 11次
	callsPerDay := 1
	if !useLegacyRandom {
		callsPerDay = 11
	}

	totalDays := 0

	// 所有条件天数总和
	for _, condition := range f.Conditions {
		startAbs := Core.DateToAbsoluteDay(condition.StartYear, condition.StartSeason, condition.StartDay)
		endAbs := Core.DateToAbsoluteDay(condition.EndYear, condition.EndSeason, condition.EndDay)

		totalDays += endAbs - startAbs + 1
	}

	// 总计算次数
	return totalDays * callsPerDay
}

func (f *FairyPredictor) GetConfigDescription() string {
	//TODO implement me
	panic("implement me")
}

func NewFairyPredictor() *FairyPredictor {
	return &FairyPredictor{
		Conditions: []FairyCondition{},
		Name:       "仙子预测",
		IsEnabled:  false,
	}
}

func (f *FairyPredictor) HasFairy(gameID, day int32, useLegacyRandom bool) bool {
	var rng *rand.Rand

	seed := Core.GetRandomSeed(day+1, gameID/2, 0, 0, 0, useLegacyRandom)
	rng = rand.New(rand.NewSource(int64(seed)))
	// 跳过前10次随机数
	for i := 0; i < 10; i++ {
		_ = rng.Float64()
	}
	// 判断概率
	return rng.Float64() < 0.01
}

// FairyDay 仙子出现日期
type FairyDay struct {
	Year   int `json:"year"`
	Season int `json:"season"`
	Day    int `json:"day"`
}

// GetFairyDays 记录条件里的天数，用于种子简介
func (f *FairyPredictor) GetFairyDays(seed int, useLegacyRandom bool) []FairyDay {
	fairyDays := []FairyDay{}

	for _, condition := range f.Conditions {
		startAbs := Core.DateToAbsoluteDay(condition.StartYear, condition.StartSeason, condition.StartDay)
		endAbs := Core.DateToAbsoluteDay(condition.EndYear, condition.EndSeason, condition.EndDay)

		for day := startAbs; day <= endAbs; day++ {
			year, season, dayOfMonth := Core.AbsoluteDayToDate(day)
			if season >= 3 { // 跳过冬天
				continue
			}

			if f.HasFairy(int32(seed), int32(day), useLegacyRandom) {
				fairyDays = append(fairyDays, FairyDay{
					Year:   year,
					Season: season,
					Day:    dayOfMonth,
				})
			}
		}
	}
	return fairyDays
}
