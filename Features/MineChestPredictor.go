package Features

import (
	"StardewSeedSearcher/Core"
	"StardewSeedSearcher/Data"
	"math/rand"
	"strconv"
	"strings"
)

// MineChestCondition 矿井宝箱的条件
type MineChestCondition struct {
	Floor    int    `json:"floor"`
	ItemName string `json:"itemName"`
}

type MineChestPredictor struct {
	Conditions []MineChestCondition `json:"conditions"`
	Name       string               `json:"name"`
	IsEnabled  bool                 `json:"isEnabled"`
}

// NewMineChestPredictor 初始化
func NewMineChestPredictor(conditions []MineChestCondition) *MineChestPredictor {
	return &MineChestPredictor{
		Conditions: []MineChestCondition{},
		Name:       "矿井宝箱",
		IsEnabled:  false,
	}
}

func (m *MineChestPredictor) GetName() string {
	return m.Name
}

func (m *MineChestPredictor) GetIsEnabled() bool {
	return m.IsEnabled
}

func (m *MineChestPredictor) SetIsEnabled(enabled bool) {
	m.IsEnabled = enabled
}

func (m *MineChestPredictor) Check(gameID int32, useLegacyRandom bool) bool {
	if !m.IsEnabled {
		return true
	}

	for _, condition := range m.Conditions {
		actualItem := m.PredictItem(int32(gameID), int32(condition.Floor), useLegacyRandom)
		if actualItem != condition.ItemName {
			return false
		}
	}
	return true
}

func (m *MineChestPredictor) EstimateCost(useLegacyRandom bool) int {
	return len(m.Conditions)
}

// GetConfigDescription 获取配置描述（用于显示当前设置）
func (m *MineChestPredictor) GetConfigDescription() string {
	if !m.IsEnabled || len(m.Conditions) == 0 {
		return "未启用"
	}

	descriptions := make([]string, len(m.Conditions))
	for i, c := range m.Conditions {
		descriptions[i] = strconv.Itoa(c.Floor) + "层:" + c.ItemName
	}
	return strings.Join(descriptions, ", ")
}

// SetConditions 设置条件（从前端请求传入）
func (m *MineChestPredictor) SetConditions(conditions []MineChestCondition) {
	if conditions == nil {
		m.Conditions = []MineChestCondition{}
	} else {
		m.Conditions = conditions
	}
	m.IsEnabled = len(m.Conditions) > 0
}

func (m *MineChestPredictor) PredictItem(gameID, floor int32, useLegacyRandom bool) string {
	var seed int32
	if useLegacyRandom {
		var temp int64 = int64(gameID)*512 + int64(floor)
		safeValue := temp % 2147483647
		// 修复：最后一个参数必须是 true
		seed = Core.GetRandomSeed(int32(safeValue), floor, 0, 0, 0, true)
	} else {
		// 修复：先转换成 int64 再乘 512，防止 32位溢出
		var temp int64 = int64(gameID) * 512
		safeValue := temp % 2147483647
		seed = Core.GetRandomSeed(int32(safeValue), floor, 0, 0, 0, false)
	}
	rng := rand.New(rand.NewSource(int64(seed)))
	var items []string = Data.ItemsCN[int(floor)]
	index := rng.Intn(len(items))
	return items[index]
}

type MineChestDetail struct {
	Floor   int    `json:"floor"`
	Item    string `json:"item"`
	Matched bool   `json:"matched"`
}

func (m *MineChestPredictor) GetDetails(gameID int32, useLegacyRandom bool) []MineChestDetail {
	results := make([]MineChestDetail, 0, len(m.Conditions))
	for _, condition := range m.Conditions {
		actualItem := m.PredictItem(gameID, int32(condition.Floor), useLegacyRandom)
		results = append(results, MineChestDetail{
			Floor:   condition.Floor,
			Item:    actualItem,
			Matched: actualItem == condition.ItemName,
		})
	}
	return results
}
