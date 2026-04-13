package Data

import (
	_ "embed"
	"encoding/json"
)

type ItemInfo struct {
	Id        string `json:"Id"`
	Name      string `json:"Name"`
	Type      string `json:"Type"`
	Category  int    `json:"Category"`
	Price     int    `json:"Price"`
	OffLimits bool   `json:"OffLimits"`
}

var SkillBooks = []string{"星露谷年历", "鱼饵和浮漂", "樵夫周刊", "采矿月刊", "战斗季刊"}

// Objects 物品字典
var Objects = map[string]ItemInfo{}

//go:embed TravelingCartData.json
var cartDataBytes []byte // 魔法指令：Go 会在编译时自动把同目录下的 JSON 内容读取到这个变量里！

// Initialize 初始化数据
func Initialize() {
	// 直接解析内存中的字节数组，彻底告别复杂的路径拼接和文件读取！
	err := json.Unmarshal(cartDataBytes, &Objects)
	if err != nil {
		// 如果解析失败，直接让程序崩溃报错，绝不能带着错误的数据继续跑！
		panic("加载猪车 JSON 数据失败: " + err.Error())
	}
}
