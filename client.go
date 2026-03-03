package migration

import (
	"encoding/json"
	"log"

	"github.com/HBulgat/migration-sdk-go/diff"
	"github.com/HBulgat/migration-sdk-go/enums"
	"github.com/HBulgat/migration-sdk-go/grayscale"
	"github.com/HBulgat/migration-sdk-go/provider"
	"github.com/HBulgat/migration-sdk-go/strategy"
)

// Config 迁移配置结构体
type Config struct {
	MigrationKey   string
	DiffServiceUrl string // 独立 Diff 服务的地址，例如 "http://diff-service:8081"
	AdminUrl       string // Admin API 的地址，用于拉取配置，例如 "http://admin-api:8080"
}

// Client 迁移客户端
type Client struct {
	config        *Config
	provider      provider.ConfigProvider
	reporter      *diff.Reporter
	factory       *strategy.Factory
	grayRuleCache []grayscale.GrayRule
	statusCache   enums.MigrationStatus
}

// NewClient 创建迁移客户端
func NewClient(config *Config) *Client {
	prv := provider.NewAPIProvider(config.AdminUrl)
	reporter := diff.NewReporter(config.DiffServiceUrl)

	client := &Client{
		config:   config,
		provider: prv,
		reporter: reporter,
	}

	client.initFactory()
	return client
}

// initFactory 初始化路由工厂并加载配置（简写为同步一次加载，实际应该有定时刷新或基于 Nacos 监听）
func (c *Client) initFactory() {
	c.factory = &strategy.Factory{
		Reporter: c.reporter, // TODO: 将注入到 Factory 中
	}

	// 拉取并刷新缓存
	if status, err := c.provider.GetStatus(c.config.MigrationKey); err == nil {
		c.statusCache = enums.MigrationStatus(status)
	} else {
		c.statusCache = enums.Old // 出错默认降级走旧逻辑
		log.Printf("[Migration-SDK] Failed to get status for %s, falling back to Old", c.config.MigrationKey)
	}

	if ruleStr, err := c.provider.GetGrayRules(c.config.MigrationKey); err == nil && ruleStr != "" {
		// Mock parse
		var rules []grayscale.GrayRule
		// 为了配合 /api/v1/grayscale_rule/list 结构，实际上要解析 PageResult。这里简写直接当作数组解析
		// 在真实的场景中应根据 Admin 层确切的 Data 结构进行反序列化
		// 以下是假设直接获取到了 Array 的伪代码:
		json.Unmarshal([]byte(ruleStr), &rules)
		c.grayRuleCache = rules
	}

	c.factory.GrayMatcher = &grayscale.DefaultMatcher{Rules: c.grayRuleCache}
}

// ExecuteFunction 提供业务层调用的实例
type ExecuteFunction struct {
	client       *Client
	oldFunc      strategy.TargetFunc
	newFunc      strategy.TargetFunc
	fallbackFunc strategy.FallbackFunc
	paramHandler strategy.ParamHandler
}

// Wrap 包装配置
func (c *Client) Wrap(oldFunc, newFunc strategy.TargetFunc, fallbackFunc strategy.FallbackFunc, paramHandler strategy.ParamHandler) *ExecuteFunction {
	return &ExecuteFunction{
		client:       c,
		oldFunc:      oldFunc,
		newFunc:      newFunc,
		fallbackFunc: fallbackFunc,
		paramHandler: paramHandler,
	}
}

// Execute 执行包装了状态路由逻辑的方法
func (e *ExecuteFunction) Execute(args ...interface{}) (interface{}, error) {
	// 每次执行时从工厂获取对应的状态策略（支持实时变化）
	s, err := e.client.factory.GetStrategy(e.client.statusCache)
	if err != nil {
		s, _ = e.client.factory.GetStrategy(enums.Old) // 降级兜底
	}

	// Delegate 执行
	return s.Execute(e.oldFunc, e.newFunc, e.fallbackFunc, e.paramHandler, e.client.config.MigrationKey, args...)
}
