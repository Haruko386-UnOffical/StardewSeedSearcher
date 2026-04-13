package Features

import (
	"StardewSeedSearcher/Core"
	"math/rand"
	"strings"
)

// DesertFestivalPredictor 沙漠节商人预测器 预测春季15-17日（沙漠节）每天的2个摊贩村民
type DesertFestivalPredictor struct {
	Name        string `json:"name"`
	IsEnabled   bool   `json:"isEnabled"`
	RequireJas  bool   `json:"requireJas"`
	RequireLeah bool   `json:"requireLeah"`
}

// GetName 实现 ISearchFeature 接口
func (d *DesertFestivalPredictor) GetName() string {
	return d.Name
}

// GetIsEnabled 实现 ISearchFeature 接口
func (d *DesertFestivalPredictor) GetIsEnabled() bool {
	return d.IsEnabled
}

// SetIsEnabled 设置是否启用
func (d *DesertFestivalPredictor) SetIsEnabled(enabled bool) {
	d.IsEnabled = enabled
}

func NewDesertFestivalPredictor() *DesertFestivalPredictor {
	return &DesertFestivalPredictor{
		Name:        "沙漠节",
		IsEnabled:   false,
		RequireJas:  false,
		RequireLeah: false,
	}
}

// 27个有资格成为沙漠节商人的村民
var POSSIBLE_VENDORS map[string]bool = map[string]bool{
	"Abigail": true, "Caroline": true, "Clint": true, "Demetrius": true,
	"Elliott": true, "Emily": true, "Evelyn": true, "George": true,
	"Gus": true, "Haley": true, "Harvey": true, "Jas": true, "Jodi": true,
	"Alex": true, "Kent": true, "Leah": true, "Marnie": true, "Maru": true,
	"Pam": true, "Penny": true, "Pierre": true, "Robin": true, "Sam": true,
	"Sebastian": true, "Shane": true, "Vincent": true, "Leo": true,
}

// 每日排除规则（春15/16/17 对应 day 0/1/2）
var SCHEDULE_EXCLUSION map[int]map[string]bool = map[int]map[string]bool{
	0: map[string]bool{"Abigail": true, "Caroline": true, "Elliott": true, "Gus": true, "Alex": true, "Leah": true, "Pierre": true, "Sam": true, "Sebastian": true, "Haley": true},
	1: map[string]bool{"Haley": true, "Clint": true, "Demetrius": true, "Maru": true, "Pam": true, "Penny": true, "Robin": true, "Leo": true},
	2: map[string]bool{"Evelyn": true, "George": true, "Jas": true, "Jodi": true, "Kent": true, "Marnie": true, "Shane": true, "Vincent": true},
}

// 角色固定顺序（来自游戏存档的角色列表顺序）
var CHARACTERS_IN_ORDER []string = []string{
	"Evelyn", "George", "Alex", "Emily", "Haley", "Jodi", "Sam", "Vincent",
	"Clint", "Lewis", "Abigail", "Caroline", "Pierre", "Gus", "Pam", "Penny",
	"Harvey", "Elliott", "Demetrius", "Maru", "Robin", "Sebastian", "Linus",
	"Wizard", "Jas", "Marnie", "Shane", "Leah", "Dwarf", "Sandy", "Willy",
}

func (d *DesertFestivalPredictor) Check(gameID int32, useLegacyRandom bool) bool {
	if !d.IsEnabled {
		return true
	}
	return d.MeetVendorsRequirement(gameID, useLegacyRandom, d.RequireJas, d.RequireLeah)
}

// EstimateCost 估算最坏情况的随机数调用次数
// 沙漠节需要预测3天，每天最多调用随机数的次数：
// 第0天: 2次（选2个商人）
// 第1天: 2次预移除 + 2次选择 = 4次
// 第2天: 4次预移除 + 2次选择 = 6次
// 总计: 2 + 4 + 6 = 12次
func (d *DesertFestivalPredictor) EstimateCost(useLegacyRandom bool) int {
	return 12
}

// GetConfigDescription 获取配置说明
func (d *DesertFestivalPredictor) GetConfigDescription() string {
	if !d.IsEnabled {
		return "未启用"
	}

	var requirements []string
	if d.RequireJas {
		requirements = append(requirements, "贾斯")
	}
	if d.RequireLeah {
		requirements = append(requirements, "莉亚")
	}

	if len(requirements) == 0 {
		return "无筛选条件"
	}

	return "第一年沙漠节需要: " + strings.Join(requirements, "、")
}

// PredictVendors 预测第一年沙漠节三天的商人 return 字典，key为0/1/2（对应春15/16/17），value为2个商人名字的列表
func (d *DesertFestivalPredictor) PredictVendors(gameID int, useLegacyRandom bool) map[int][]string {
	var vendors = map[int][]string{
		0: {},
		1: {},
		2: {},
	}
	// 遍历三天（春15/16/17）
	for i := 0; i < 3; i++ {
		day := 15 + i
		// 构建当天的候选池
		vendorPool := d.BuildVendorPool(i)
		// 初始化RNG
		seed := Core.GetRandomSeed(int32(day), int32(gameID/2), 0, 0, 0, useLegacyRandom)
		rng := rand.New(rand.NewSource(int64(seed)))

		// 预移除：跨日去重逻辑
		// 第0天移除0个，第1天移除2个，第2天移除4个
		for k := 0; k < i; k++ {
			for m := 0; m < 2; m++ {
				index := rng.Intn(len(vendorPool))
				vendorPool = removeAtIndex(vendorPool, index)
			}
		}

		// 选择当天的2个商人
		for j := 0; j < 2; j++ {
			index := rng.Intn(len(vendorPool))
			selectedVendor := vendorPool[index]
			vendors[i] = append(vendors[i], selectedVendor)
			vendorPool = removeAtIndex(vendorPool, index)
		}
	}
	return vendors
}

func (d *DesertFestivalPredictor) MeetVendorsRequirement(gameID int32, useLegacyRandom bool, requireJas, requireLeah bool) bool {
	// 如果两个都不勾选，直接返回 true（不筛选）
	if !requireLeah && !requireJas {
		return true
	}

	// 预测三天商人
	vendors := d.PredictVendors(int(gameID), useLegacyRandom)

	// 检查是否满足条件
	hasJas := false
	hasLeah := false

	for _, dayVendors := range vendors {
		for _, vendor := range dayVendors {
			if vendor == "Jas" {
				hasJas = true
			} else if vendor == "Leah" {
				hasLeah = true
			}
		}
	}

	if requireJas && !hasJas {
		return false
	}
	if requireLeah && !hasLeah {
		return false
	}
	return true
}

// BuildVendorPool 构建指定日期的候选村民池 i 相对日期: 0=春15, 1=春16, 2=春17 return 候选村民列表（保持固定顺序）
func (d *DesertFestivalPredictor) BuildVendorPool(i int) []string {
	pool := make([]string, 0)
	exclusion := SCHEDULE_EXCLUSION[i]

	for _, name := range CHARACTERS_IN_ORDER {
		// 检查是否在候选名单中 || 检查是否被当天排除
		if !POSSIBLE_VENDORS[name] || exclusion[name] {
			continue
		}
		pool = append(pool, name)
	}
	return pool
}

// VendorDetail 商人详情结构体（用于JSON序列化）
type VendorDetail struct {
	Day15 []string `json:"day15"`
	Day16 []string `json:"day16"`
	Day17 []string `json:"day17"`
}

// GetVendorDetail 获取结构化的商人详情
func (d *DesertFestivalPredictor) GetVendorDetail(seed int, useLegacyRandom bool) map[string][]string {
	vendors := d.PredictVendors(seed, useLegacyRandom)

	return map[string][]string{
		"day15": vendors[0],
		"day16": vendors[1],
		"day17": vendors[2],
	}
}

// removeAtIndex 从切片中移除指定索引的元素
func removeAtIndex(slice []string, index int) []string {
	if index < 0 || index >= len(slice) {
		return slice
	}
	return append(slice[:index], slice[index+1:]...)
}
