package migration

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/HBulgat/migration-sdk-go/diff"
	"github.com/HBulgat/migration-sdk-go/enums"
	"github.com/HBulgat/migration-sdk-go/grayscale"
	"github.com/HBulgat/migration-sdk-go/provider"
	"github.com/HBulgat/migration-sdk-go/strategy"
)

// Config 迁移配置结构体
type Config struct {
	DiffServiceUrl string // 独立 Diff 服务的地址，例如 "http://diff-service:8081"
	AdminUrl       string // Admin API 的地址，用于拉取配置，例如 "http://admin-api:8080"
}

// Client 迁移客户端
type Client struct {
	config   *Config
	provider provider.ConfigProvider
	reporter *diff.Reporter
	factory  *strategy.Factory

	mu            sync.RWMutex
	grayRuleCache map[string][]grayscale.GrayRule
	statusCache   map[string]enums.MigrationStatus
}

// NewClient 创建迁移客户端
func NewClient(config *Config) *Client {
	prv := provider.NewAPIProvider(config.AdminUrl)
	reporter := diff.NewReporter(config.DiffServiceUrl)

	client := &Client{
		config:        config,
		provider:      prv,
		reporter:      reporter,
		grayRuleCache: make(map[string][]grayscale.GrayRule),
		statusCache:   make(map[string]enums.MigrationStatus),
	}

	client.initFactory()
	return client
}

// GetRules 实现 grayscale.RuleProvider 接口
func (c *Client) GetRules(migrationKey string) []grayscale.GrayRule {
	c.mu.RLock()
	rules, exists := c.grayRuleCache[migrationKey]
	c.mu.RUnlock()

	if exists {
		return rules
	}
	return nil
}

// initFactory 初始化路由工厂
func (c *Client) initFactory() {
	c.factory = &strategy.Factory{
		Reporter:    c.reporter,
		GrayMatcher: &grayscale.DefaultMatcher{RuleProvider: c},
	}
}

// loadConfigIfAbsent 确保加载指定 migrationKey 的缓存
func (c *Client) loadConfigIfAbsent(migrationKey string) {
	c.mu.RLock()
	_, statusOK := c.statusCache[migrationKey]
	c.mu.RUnlock()

	if statusOK {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double check
	if _, ok := c.statusCache[migrationKey]; ok {
		return
	}

	if status, err := c.provider.GetStatus(migrationKey); err == nil {
		c.statusCache[migrationKey] = enums.MigrationStatus(status)
	} else {
		c.statusCache[migrationKey] = enums.Old // 出错默认降级走旧逻辑
		log.Printf("[Migration-SDK] Failed to get status for %s, falling back to Old", migrationKey)
	}

	if ruleStr, err := c.provider.GetGrayRules(migrationKey); err == nil && ruleStr != "" {
		var rules []grayscale.GrayRule
		json.Unmarshal([]byte(ruleStr), &rules)
		c.grayRuleCache[migrationKey] = rules
	} else {
		c.grayRuleCache[migrationKey] = nil
	}
}

// ExecuteFunction 提供业务层调用的实例
type ExecuteFunction struct {
	client       *Client
	migrationKey string
	oldFunc      strategy.TargetFunc
	newFunc      strategy.TargetFunc
	fallbackFunc strategy.FallbackFunc
	paramHandler strategy.ParamHandler
}

// Wrap 包装配置
func (c *Client) Wrap(migrationKey string, oldFunc, newFunc strategy.TargetFunc, fallbackFunc strategy.FallbackFunc, paramHandler strategy.ParamHandler) *ExecuteFunction {
	c.loadConfigIfAbsent(migrationKey)

	return &ExecuteFunction{
		client:       c,
		migrationKey: migrationKey,
		oldFunc:      oldFunc,
		newFunc:      newFunc,
		fallbackFunc: fallbackFunc,
		paramHandler: paramHandler,
	}
}

// Execute 执行包装了状态路由逻辑的方法
func (e *ExecuteFunction) Execute(args ...interface{}) (interface{}, error) {
	e.client.mu.RLock()
	status := e.client.statusCache[e.migrationKey]
	e.client.mu.RUnlock()

	// 每次执行时从工厂获取对应的状态策略（支持实时变化）
	s, err := e.client.factory.GetStrategy(status)
	if err != nil {
		s, _ = e.client.factory.GetStrategy(enums.Old) // 降级兜底
	}

	// Delegate 执行
	return s.Execute(e.oldFunc, e.newFunc, e.fallbackFunc, e.paramHandler, e.migrationKey, args...)
}
