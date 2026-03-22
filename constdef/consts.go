package constdef

// MigrationTaskStatus 迁移状态枚举
// 0-未知(MigrationTaskStatusUnknown)
// 1-单旧(MigrationTaskStatusOld)：只调用旧接口
// 2-验证-灰度(MigrationTaskStatusValidationGray)：并发调用，进行Diff，返回旧接口结果
// 3-验证-全开(MigrationTaskStatusValidationAll)：并发调用，进行Diff，返回旧接口结果
// 4-上线-灰度(MigrationTaskStatusGoLiveGray)：并发调用，进行Diff，根据灰度规则返回
// 5-上线-全开(MigrationTaskStatusGoLiveAll)：并发调用，进行Diff，返回新接口结果
// 6-停用-灰度(MigrationTaskStatusDecommissioningGray)：根据灰度规则调用新接口或并发调用
// 7-停用-全开(MigrationTaskStatusDecommissioningAll)：只调用新接口
type MigrationTaskStatus int

const (
	MigrationTaskStatusUnknown             MigrationTaskStatus = 0
	MigrationTaskStatusOld                 MigrationTaskStatus = 1
	MigrationTaskStatusValidationGray      MigrationTaskStatus = 2
	MigrationTaskStatusValidationAll       MigrationTaskStatus = 3
	MigrationTaskStatusGoLiveGray          MigrationTaskStatus = 4
	MigrationTaskStatusGoLiveAll           MigrationTaskStatus = 5
	MigrationTaskStatusDecommissioningGray MigrationTaskStatus = 6
	MigrationTaskStatusDecommissioningAll  MigrationTaskStatus = 7
)

type GrayRuleType string

const (
	GrayRuleTypePercentage GrayRuleType = "PERCENTAGE"
	GrayRuleTypeBlackList  GrayRuleType = "BLACKLIST"
	GrayRuleTypeWhiteList  GrayRuleType = "WHITELIST"
	GrayRuleTypeExpression GrayRuleType = "EXPRESSION"
)
