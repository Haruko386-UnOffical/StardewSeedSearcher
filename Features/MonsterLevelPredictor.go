package Features

import (
	"StardewSeedSearcher/Core"
	"fmt"
	"math/rand"
	"strings"
)

type MonsterLevelCondition struct {
	StartSeason int `json:"startSeason"` // 补上季节
	EndSeason   int `json:"endSeason"`   // 补上季节
	StartDay    int `json:"startDay"`
	EndDay      int `json:"endDay"`
	StartLevel  int `json:"startLevel"`
	EndLevel    int `json:"endLevel"`
}

type MonsterLevelPredictor struct {
	Conditions []MonsterLevelCondition `json:"conditions"`
	Name       string                  `json:"name"`
	IsEnabled  bool                    `json:"isEnabled"`
}

func (m *MonsterLevelPredictor) GetName() string {
	return m.Name
}

func (m *MonsterLevelPredictor) GetIsEnabled() bool {
	return m.IsEnabled
}

func (m *MonsterLevelPredictor) Check(gameID int32, useLegacyRandom bool) bool {
	if !m.IsEnabled {
		return true
	}

	for _, condition := range m.Conditions {
		// 先计算绝对天数（假设固定为第一年）
		absoluteStartDay := Core.DateToAbsoluteDay(1, condition.StartSeason, condition.StartDay)
		absoluteEndDay := Core.DateToAbsoluteDay(1, condition.EndSeason, condition.EndDay)

		for day := absoluteStartDay; day <= absoluteEndDay; day++ {
			for level := condition.StartLevel; level <= condition.EndLevel; level++ {
				// 跳过电梯层
				if level%5 == 0 {
					continue
				}

				var rng *rand.Rand
				if useLegacyRandom {
					// 注意：这里用的是绝对天数 day
					seed := int32(day) + int32(level*100) + gameID/2
					rng = rand.New(rand.NewSource(int64(seed)))
				} else {
					seed := Core.GetRandomSeed(int32(day), int32(level), gameID/2, 0, 0, false)
					rng = rand.New(rand.NewSource(int64(seed)))
				}

				if rng.Float64() < 0.05 {
					continue
				}

				// 只要有一天某一层符合要求，这个条件就不满足
				return false
			}
		}
	}
	return true
}

// EstimateCost 估算搜索成本
func (m *MonsterLevelPredictor) EstimateCost(useLegacyRandom bool) int {
	totalCost := 0
	for _, condition := range m.Conditions {
		days := condition.EndDay - condition.StartDay + 1
		levels := condition.EndLevel - condition.StartLevel + 1
		elevatorCount := 0
		for level := condition.StartLevel; level <= condition.EndLevel; level++ {
			if level%5 == 0 {
				elevatorCount++
			}
		}
		totalCost += days * (levels - elevatorCount)
	}
	return totalCost
}

// GetConfigDescription 获取配置描述（用于显示当前设置）
func (m *MonsterLevelPredictor) GetConfigDescription() string {
	if !m.IsEnabled || len(m.Conditions) == 0 {
		return "未启用"
	}

	descriptions := make([]string, len(m.Conditions))
	for i, c := range m.Conditions {
		descriptions[i] = m.FormatConditionDescription(c)
	}
	return strings.Join(descriptions, ", ")
}

func NewMonsterLevelPredictor() *MonsterLevelPredictor {
	return &MonsterLevelPredictor{
		Conditions: []MonsterLevelCondition{},
		Name:       "怪物层",
		IsEnabled:  false,
	}
}

// SetConditions 设置条件（从前端请求传入）
func (m *MonsterLevelPredictor) SetConditions(conditions []MonsterLevelCondition) {
	if conditions == nil {
		m.Conditions = []MonsterLevelCondition{}
	} else {
		m.Conditions = conditions
	}
	m.IsEnabled = len(m.Conditions) > 0
}

func (m *MonsterLevelPredictor) SetName(name string) {
	m.Name = name
}

func (m *MonsterLevelPredictor) FormatConditionDescription(c MonsterLevelCondition) string {
	seasonNames := []string{"春", "夏", "秋", "冬"}
	startSeason := (c.StartDay - 1) / 28
	startDayOfMonth := ((c.StartDay - 1) % 28) + 1
	endSeason := (c.EndDay - 1) / 28
	endDayOfMonth := ((c.EndDay - 1) % 28) + 1

	// 构建日期范围字符串
	var dateRange string
	if c.StartDay == c.EndDay {
		dateRange = fmt.Sprintf("%s%d", seasonNames[startSeason], startDayOfMonth)
	} else {
		dateRange = fmt.Sprintf("%s%d-%s%d",
			seasonNames[startSeason], startDayOfMonth,
			seasonNames[endSeason], endDayOfMonth)
	}

	// 返回完整描述
	return fmt.Sprintf("%s %d-%d层无怪物层", dateRange, c.StartLevel, c.EndLevel)
}

// GetDetails 获取详细信息（用于结果展示）
func (m *MonsterLevelPredictor) GetDetails(gameID int, useLegacyRandom bool) []map[string]interface{} {
	results := make([]map[string]interface{}, len(m.Conditions))
	for i, c := range m.Conditions {
		results[i] = map[string]interface{}{
			"description": m.FormatConditionDescription(c),
			"satisfied":   true,
		}
	}
	return results
}
