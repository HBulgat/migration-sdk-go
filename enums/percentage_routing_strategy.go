package enums

// PercentageRoutingStrategy 定义百分比路由策略
type PercentageRoutingStrategy string

const (
	// StrategyHash 使用请求字段哈希值保证同请求稳定命中
	StrategyHash PercentageRoutingStrategy = "HASH"
	// StrategyRandom 使用随机数生成器进行路由
	StrategyRandom PercentageRoutingStrategy = "RANDOM"
)
