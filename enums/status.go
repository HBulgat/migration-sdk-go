package enums

// MigrationStatus 迁移状态枚举
// 1-单旧(Old)：只调用旧接口
// 2-验证-灰度(Validation-gray)：并发调用，进行Diff，返回旧接口结果
// 3-验证-全开(Validation-all)：并发调用，进行Diff，返回旧接口结果
// 4-上线-灰度(Go-Live-gray)：并发调用，进行Diff，根据灰度规则返回
// 5-上线-全开(Go-Live-all)：并发调用，进行Diff，返回新接口结果
// 6-停用-灰度(Decommissioning-gray)：根据灰度规则调用新接口或并发调用
// 7-停用-全开(Decommissioning-all)：只调用新接口
type MigrationStatus int

const (
	Old                 MigrationStatus = 1
	ValidationGray      MigrationStatus = 2
	ValidationAll       MigrationStatus = 3
	GoLiveGray          MigrationStatus = 4
	GoLiveAll           MigrationStatus = 5
	DecommissioningGray MigrationStatus = 6
	DecommissioningAll  MigrationStatus = 7
)
