package Features

type (
	// ISearchFeature 搜种器功能接口 所有筛选功能都实现此接口
	ISearchFeature interface {
		GetName() string                               // 功能名称
		GetIsEnabled() bool                            //是否启用此功能
		Check(gameID int32, useLegacyRandom bool) bool //检查种子是否符合此功能的筛选条件 gameID: 游戏种子  useLegacyRandom 是否使用旧随机
		EstimateCost(useLegacyRandom bool) int         // 估算最坏情况的随机数调用次数，用于动态成本计算
		GetConfigDescription() string                  // 获取功能的配置说明
	}
)
