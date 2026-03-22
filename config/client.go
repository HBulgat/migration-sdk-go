package config

import (
	"github.com/HBulgat/migration-sdk-go/constdef"
	"github.com/HBulgat/migration-sdk-go/gray"
)

// ConfigClient 配置获取接口，SDK 预留了从不同途径（API接口或直接读取配置中心）获取配置的扩展点
type ConfigClient interface {
	// GetStatus 获取当前迁移任务的迁移状态
	GetStatus(migrationKey string) (constdef.MigrationTaskStatus, error)
	// GetGrayRules 获取当前迁移任务的灰度规则
	GetGrayRules(migrationKey string) ([]gray.GrayRule, error)
}
