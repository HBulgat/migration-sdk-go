package provider

// ConfigProvider 配置获取接口，SDK 预留了从不同途径（API接口或直接读取配置中心）获取配置的扩展点
type ConfigProvider interface {
	// GetStatus 获取当前迁移任务的迁移状态
	GetStatus(migrationKey string) (int, error)
	// GetGrayRules 获取当前迁移任务的灰度规则，返回规则 JSON 字符串
	GetGrayRules(migrationKey string) (string, error)
	// GetDiffRules 获取当前迁移任务的 Diff 规则，返回规则 JSON 字符串
	GetDiffRules(migrationKey string) (string, error)
}
