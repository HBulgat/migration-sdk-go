package grayscale

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"sync"

	"github.com/HBulgat/migration-sdk-go/enums"
	"github.com/jmespath/go-jmespath"
)

// Matcher 灰度匹配器接口
type Matcher interface {
	Match(migrationKey string, params map[string]interface{}) bool
}

// RuleProvider 提供规则获取的接口
type RuleProvider interface {
	GetRules(migrationKey string) []GrayRule
}

// DefaultMatcher 默认灰度匹配器实现
type DefaultMatcher struct {
	RuleProvider              RuleProvider
	exprCache                 sync.Map // string -> *jmespath.JMESPath
	PercentageRoutingStrategy enums.PercentageRoutingStrategy
}

// NewDefaultMatcher 创建灰度匹配器
func NewDefaultMatcher(ruleProvider RuleProvider, strategy enums.PercentageRoutingStrategy) *DefaultMatcher {
	if strategy == "" {
		strategy = enums.StrategyHash
	}
	return &DefaultMatcher{
		RuleProvider:              ruleProvider,
		PercentageRoutingStrategy: strategy,
	}
}

type GrayRule struct {
	RuleType  string `json:"rule_type"`
	RuleValue string `json:"rule_value"`
	Enable    bool   `json:"enable"`
	Weight    int    `json:"weight"`
}

func (m *DefaultMatcher) Match(migrationKey string, params map[string]interface{}) bool {
	rules := m.RuleProvider.GetRules(migrationKey)
	if len(rules) == 0 {
		return false
	}

	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].Weight > rules[j].Weight
	})

	for _, rule := range rules {
		if !rule.Enable {
			continue
		}

		switch rule.RuleType {
		case "PERCENTAGE":
			if matchPercentage(rule.RuleValue, params, m.PercentageRoutingStrategy) {
				return true
			}
		case "BLACKLIST":
			if matchList(rule.RuleValue, params, false) {
				// 黑名单逻辑：若在此名单中，则明确拒绝参与新链路 (强制 false)
				// （实际情况可能更复杂，这里做演示简写，匹配黑名单即为未命中）
				return false
			}
		case "WHITELIST":
			if matchList(rule.RuleValue, params, true) {
				return true
			}
		case "EXPRESSION":
			if m.matchExpression(rule.RuleValue, params) {
				return true
			}
		}
	}
	return false
}

func (m *DefaultMatcher) matchExpression(expr string, params map[string]interface{}) bool {
	var jm *jmespath.JMESPath
	if cached, ok := m.exprCache.Load(expr); ok {
		jm = cached.(*jmespath.JMESPath)
	} else {
		compiled, err := jmespath.Compile(expr)
		if err != nil {
			log.Printf("[Migration-SDK] Failed to compile JMESPath expression '%s': %v", expr, err)
			return false
		}
		jm = compiled
		m.exprCache.Store(expr, jm)
	}

	result, err := jm.Search(params)
	if err != nil {
		log.Printf("[Migration-SDK] Failed to evaluate JMESPath expression '%s': %v", expr, err)
		return false
	}

	if b, ok := result.(bool); ok {
		return b
	}
	return false
}

func extractSubject(params map[string]interface{}) string {
	keys := []string{"userId", "user_id", "uid", "id"}
	for _, key := range keys {
		if val, ok := params[key]; ok && val != nil {
			return fmt.Sprintf("%v", val)
		}
	}
	b, _ := json.Marshal(params)
	return string(b)
}

func javaStringHashCode(s string) int32 {
	var h int32
	for i := 0; i < len(s); i++ {
		h = 31*h + int32(s[i])
	}
	return h
}

func absolute(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

func matchPercentage(value string, params map[string]interface{}, strategy enums.PercentageRoutingStrategy) bool {
	percentage, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	if percentage <= 0 {
		return false
	}
	if percentage >= 100 {
		return true
	}

	if strategy == enums.StrategyRandom {
		return rand.Intn(100) < percentage
	}

	subject := extractSubject(params)
	bucket := absolute(javaStringHashCode(subject)) % 100
	return bucket < int32(percentage)
}

func matchList(value string, params map[string]interface{}, isWhitelist bool) bool {
	var list []string
	if err := json.Unmarshal([]byte(value), &list); err != nil {
		return false
	}

	// 这里简写：默认只匹配 userId。实际应该做更通用的匹配或参数绑定
	paramVal, ok := params["userId"]
	if !ok {
		return false
	}
	strVal := ""
	switch v := paramVal.(type) {
	case string:
		strVal = v
	case int:
		strVal = strconv.Itoa(v)
	}

	for _, item := range list {
		if item == strVal {
			return true
		}
	}
	return false
}
