package migration

import (
	"context"
	"time"

	configclient "github.com/HBulgat/migration-sdk-go/config"
	"github.com/HBulgat/migration-sdk-go/constdef"
	"github.com/HBulgat/migration-sdk-go/diff"
	"github.com/HBulgat/migration-sdk-go/gray"
)

// Config 迁移配置结构体
type Config struct {
	ConfigCenterClientConfig *ConfigCenterClientConfig `json:"config_center_client_config"`
	DiffReporterConfig       *DiffReporterConfig       `json:"diff_reporter_config"`
}

type ConfigCenterClientConfig struct {
	Enable                      bool          `json:"enable"`
	Address                     string        `json:"address"`
	InternalToken               string        `json:"internal_token"`
	TimeoutSeconds              time.Duration `json:"timeout_seconds"`
	CacheEnable                 bool          `json:"cache_enable"`
	CacheRefreshIntervalSeconds int32         `json:"cache_refresh_interval_seconds"`
}

type DiffReporterConfig struct {
	Enable         bool          `json:"enable"`
	Address        string        `json:"address"`
	InternalToken  string        `json:"internal_token"`
	TimeoutSeconds time.Duration `json:"timeout_seconds"`
}

// Client 迁移客户端
type Client struct {
	config       *Config
	configClient *configclient.ConfigClient
	diffReporter *diff.DiffReporter
}

func CreateConfigClient(config *ConfigCenterClientConfig) configclient.ConfigClient {
	if config == nil {
		panic("config is nil")
	}
	if !config.Enable {
		return nil
	}
	configClient := configclient.NewHttpConfigClient(config.Address, config.InternalToken, config.TimeoutSeconds)
	if config.CacheEnable {
		return configclient.NewCachedConfigClient(configClient, config.CacheRefreshIntervalSeconds)
	}
	return configClient
}

func CreateDiffReporter(config *DiffReporterConfig) diff.DiffReporter {
	if config == nil {
		panic("config is nil")
	}
	if !config.Enable {
		return nil
	}
	return diff.NewHttpDiffReporter(config.Address, config.InternalToken, config.TimeoutSeconds)
}

// NewClient 创建迁移客户端
func NewClient(config *Config) *Client {
	if config == nil {
		panic("config is nil")
	}
	configClient := CreateConfigClient(config.ConfigCenterClientConfig)
	diffReporter := CreateDiffReporter(config.DiffReporterConfig)
	client := &Client{
		config:       config,
		configClient: &configClient,
		diffReporter: &diffReporter,
	}
	return client
}

func (c *Client) Wrap(migrationKey string, paramHandler ParamHandler, processor PostProcessor,
	functions ...Function) Function {
	if migrationKey != "" {
		if client, ok := (*c.configClient).(*configclient.CachedConfigClient); ok {
			client.RegistryKey(migrationKey)
		}
	}
	if len(functions) < 2 {
		panic("migration wrapped functions length must be greater than or equal to 2 (oldFunc, newFunc, [fallbackFunc])")
	}
	oldFunc := functions[0]
	newFunc := functions[1]
	var fallbackFunc Function
	if len(functions) >= 3 {
		fallbackFunc = functions[2]
	}

	return func(args ...interface{}) (interface{}, error) {
		// 框架级异常的兜底执行：当配置无法拉取或执行受阻时，优先尝试用户降级，次优直接放行旧接口
		doFrameworkFallback := func() (interface{}, error) {
			if fallbackFunc != nil {
				return fallbackFunc(args...)
			}
			return oldFunc(args...)
		}

		if c.configClient == nil {
			return doFrameworkFallback()
		}

		status, err := (*c.configClient).GetStatus(migrationKey)
		if err != nil || status == constdef.MigrationTaskStatusUnknown {
			// todo log: config pull failed or unknown status
			return doFrameworkFallback()
		}

		executeFunc := executeFunctionMap[status]
		if executeFunc == nil {
			// todo log: unsupported status
			return doFrameworkFallback()
		}

		rules, err := (*c.configClient).GetGrayRules(migrationKey)
		if err != nil {
			// todo log: config pull gray rules failed
			return doFrameworkFallback()
		}

		params := paramHandler(args...)
		ctx := &Context{
			Context:         args[0].(context.Context),
			Client:          c,
			OldFunc:         oldFunc,
			NewFunc:         newFunc,
			FallbackFunc:    fallbackFunc,
			ParamHandler:    paramHandler,
			Param:           params,
			PostProcessor:   processor,
			MigrationKey:    migrationKey,
			MigrationStatus: status,
			GrayRules:       rules,
			Args:            args,
			HitGray:         gray.Match(params, rules),
		}

		res, err := executeFunc(ctx)
		if err != nil {
			return ExecuteFallbackAfterFailed(ctx, err)
		}
		return res, nil
	}
}
