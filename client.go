package migration

import (
	"log"
	"sync"
	"time"

	"github.com/HBulgat/migration-sdk-go/diff"
	"github.com/HBulgat/migration-sdk-go/enums"
	"github.com/HBulgat/migration-sdk-go/grayscale"
	"github.com/HBulgat/migration-sdk-go/provider"
	"github.com/HBulgat/migration-sdk-go/strategy"
	"github.com/google/uuid"
)

// Config 迁移配置结构体
type Config struct {
	DiffServiceUrl            string                          `json:"diff_service_url"`            // 独立 Diff 服务的地址，例如 "http://diff-service:8081"
	AdminUrl                  string                          `json:"admin_url"`                   // Admin API 的地址，用于拉取配置，例如 "http://admin-api:8080"
	RefreshInterval           time.Duration                   `json:"refresh_interval"`            // 缓存后台刷新间隔，默认为 60s
	InternalToken             string                          `json:"internal_token"`              // Admin API 的内部鉴权 Token，例如 "MIGRATION_DEFAULT_SDK_TOKEN"
	PercentageRoutingStrategy enums.PercentageRoutingStrategy `json:"percentage_routing_strategy"` // 百分比路由策略，支持 HASH 和 RANDOM，默认为 HASH
}

// Client 迁移客户端
type Client struct {
	config        *Config
	provider      provider.ConfigProvider
	reporter      *diff.Reporter
	factory       *strategy.Factory
	mu            sync.RWMutex
	statusCache   map[string]enums.MigrationTaskStatus
	grayRuleCache map[string][]grayscale.GrayRule
	trackedKeys   map[string]bool
}

// NewClient 创建迁移客户端
func NewClient(config *Config) *Client {
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 60 * time.Second
	}
	if config.PercentageRoutingStrategy == "" {
		config.PercentageRoutingStrategy = enums.StrategyHash
	}

	prv := provider.NewAPIProvider(config.AdminUrl, config.InternalToken)
	reporter := diff.NewReporter(config.DiffServiceUrl)

	client := &Client{
		config:        config,
		provider:      prv,
		reporter:      reporter,
		statusCache:   make(map[string]enums.MigrationTaskStatus),
		grayRuleCache: make(map[string][]grayscale.GrayRule),
		trackedKeys:   make(map[string]bool),
	}

	client.initFactory()
	go client.startBackgroundRefresh()
	return client
}

// GetRules 实现 grayscale.RuleProvider 接口
func (c *Client) GetRules(migrationKey string) []grayscale.GrayRule {
	c.mu.RLock()
	rules, exists := c.grayRuleCache[migrationKey]
	c.mu.RUnlock()

	if !exists {
		c.fetchAndCache(migrationKey)
		c.mu.RLock()
		rules = c.grayRuleCache[migrationKey]
		c.mu.RUnlock()
	}

	return rules
}

// initFactory 初始化路由工厂
func (c *Client) initFactory() {
	c.factory = &strategy.Factory{
		Reporter:    c.reporter,
		GrayMatcher: grayscale.NewDefaultMatcher(c, c.config.PercentageRoutingStrategy),
	}
}

// ExecuteFunction 提供业务层调用的实例
type ExecuteFunction struct {
	client       *Client
	migrationKey string
	oldFunc      strategy.TargetFunc
	newFunc      strategy.TargetFunc
	fallbackFunc  strategy.FallbackFunc
	paramHandler  strategy.ParamHandler
	postProcessor strategy.PostProcessor
}

// RequestWrapper 用于某次具体请求的包装，保证并发安全
type RequestWrapper struct {
	executeFn *ExecuteFunction
	traceId   string
}

// WithTraceId 注入链路跟踪 ID，返回一个新的请求级别包装器，解决并发修改的 Data Race
func (e *ExecuteFunction) WithTraceId(traceId string) *RequestWrapper {
	if traceId == "" {
		traceId = uuid.NewString()
	}
	return &RequestWrapper{
		executeFn: e,
		traceId:   traceId,
	}
}

// Execute 封装一次执行，如果没有注入 TraceId，则自动生成
func (e *ExecuteFunction) Execute(args ...interface{}) (interface{}, error) {
	return e.WithTraceId("").Execute(args...)
}

// Option 定义针对 ExecuteFunction 的可选配置项
type Option func(*ExecuteFunction)

// WithPostProcessor 设置当前调用的后置数据处理器
func WithPostProcessor(p strategy.PostProcessor) Option {
	return func(e *ExecuteFunction) {
		e.postProcessor = p
	}
}

// Wrap 包装配置
func (c *Client) Wrap(migrationKey string, oldFunc, newFunc strategy.TargetFunc, fallbackFunc strategy.FallbackFunc, paramHandler strategy.ParamHandler, opts ...Option) *ExecuteFunction {
	e := &ExecuteFunction{
		client:       c,
		migrationKey: migrationKey,
		oldFunc:      oldFunc,
		newFunc:      newFunc,
		fallbackFunc: fallbackFunc,
		paramHandler: paramHandler,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Execute 执行包装了状态路由逻辑的方法
func (r *RequestWrapper) Execute(args ...interface{}) (interface{}, error) {
	e := r.executeFn
	c := e.client
	c.mu.RLock()
	status, exists := c.statusCache[e.migrationKey]
	c.mu.RUnlock()

	if !exists {
		c.fetchAndCache(e.migrationKey)
		c.mu.RLock()
		status, exists = c.statusCache[e.migrationKey]
		c.mu.RUnlock()
		if !exists {
			status = enums.Old // 网络降级兜底
		}
	}

	// 每次执行时从工厂获取对应的状态策略（支持实时变化）
	s, err := e.client.factory.GetStrategy(status)
	if err != nil {
		s, _ = e.client.factory.GetStrategy(enums.Old) // 降级兜底
	}

	// 获取规则用于上报
	rules, err := e.client.provider.GetGrayRules(e.migrationKey)
	if err != nil {
		rules = []grayscale.GrayRule{}
	}

	// Delegate 执行
	return s.Execute(e.oldFunc, e.newFunc, e.fallbackFunc, e.paramHandler, e.postProcessor, e.migrationKey, r.traceId, status, rules, args...)
}

func (c *Client) fetchAndCache(migrationKey string) {
	status, err := c.provider.GetStatus(migrationKey)
	if err != nil {
		log.Printf("[Migration-SDK] Failed to lazy load status for %s: %v", migrationKey, err)
		return
	}

	rules, err := c.provider.GetGrayRules(migrationKey)
	if err != nil {
		log.Printf("[Migration-SDK] Failed to lazy load gray rules for %s: %v", migrationKey, err)
		// We still cache the status even if rules fail
		rules = []grayscale.GrayRule{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.trackedKeys[migrationKey] = true
	c.statusCache[migrationKey] = status
	c.grayRuleCache[migrationKey] = rules
}

func (c *Client) startBackgroundRefresh() {
	ticker := time.NewTicker(c.config.RefreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.RLock()
		keys := make([]string, 0, len(c.trackedKeys))
		for k := range c.trackedKeys {
			keys = append(keys, k)
		}
		c.mu.RUnlock()

		for _, key := range keys {
			// Synchronous fetch without holding the global lock
			status, err := c.provider.GetStatus(key)
			if err != nil {
				log.Printf("[Migration-SDK] Background refresh failed for migration status. key=%s, err=%v", key, err)
				continue
			}

			rules, err := c.provider.GetGrayRules(key)
			if err != nil {
				log.Printf("[Migration-SDK] Background refresh failed for grayscale rules. key=%s, err=%v", key, err)
				continue
			}

			// Atomic update of cache
			c.mu.Lock()
			c.statusCache[key] = status
			c.grayRuleCache[key] = rules
			c.mu.Unlock()
		}
	}
}
