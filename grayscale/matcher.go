package grayscale

import (
	"encoding/json"
	"math/rand"
	"strconv"
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
	RuleProvider RuleProvider
}

type GrayRule struct {
	RuleType  string `json:"rule_type"`
	RuleValue string `json:"rule_value"`
	Enable    bool   `json:"enable"`
}

func (m *DefaultMatcher) Match(migrationKey string, params map[string]interface{}) bool {
	rules := m.RuleProvider.GetRules(migrationKey)
	if len(rules) == 0 {
		return false
	}

	for _, rule := range rules {
		if !rule.Enable {
			continue
		}

		switch rule.RuleType {
		case "PERCENTAGE":
			if matchPercentage(rule.RuleValue) {
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
			// Go 侧可能需要接入 Gval 等第三方引擎计算 SpEL
			// 这里暂时略过，按未命中处理
		}
	}
	return false
}

func matchPercentage(value string) bool {
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
	num := rand.Intn(100) // 0-99
	return num < percentage
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
