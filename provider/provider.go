package provider

import (
	"github.com/HBulgat/migration-sdk-go/enums"
	"github.com/HBulgat/migration-sdk-go/grayscale"
)

// ConfigProvider 配置获取接口，SDK 预留了从不同途径（API接口或直接读取配置中心）获取配置的扩展点
type ConfigProvider interface {
	// GetStatus 获取当前迁移任务的迁移状态
	GetStatus(migrationKey string) (enums.MigrationTaskStatus, error)
	// GetGrayRules 获取当前迁移任务的灰度规则
	GetGrayRules(migrationKey string) ([]grayscale.GrayRule, error)
}
