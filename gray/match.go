package gray

import (
	"encoding/json"
	"log"
	"math/rand"
	"strconv"
	"sync"

	"github.com/HBulgat/migration-sdk-go/constdef"
	"github.com/jmespath/go-jmespath"
)

var ExprCache sync.Map

type Rule struct {
	RuleType  string `json:"rule_type"`
	RuleValue string `json:"rule_value"`
	Enable    bool   `json:"enable"`
}

func Match(params map[string]interface{}, rules []Rule) bool {
	if len(rules) == 0 {
		return false
	}

	for _, rule := range rules {
		if !rule.Enable {
			continue
		}
		switch constdef.GrayRuleType(rule.RuleType) {
		case constdef.GrayRuleTypePercentage:
			if matchPercentage(rule.RuleValue) {
				return true
			}
		case constdef.GrayRuleTypeBlackList:
			if !matchList(rule.RuleValue, params) {
				return false // 一旦命中黑名单，直接返回false
			}
		case constdef.GrayRuleTypeWhiteList:
			if matchList(rule.RuleValue, params) {
				return true
			}
		case constdef.GrayRuleTypeExpression:
			if matchExpression(rule.RuleValue, params) {
				return true
			}
		}
	}
	return false
}

func matchExpression(expr string, params map[string]interface{}) bool {
	var jm *jmespath.JMESPath
	if cached, ok := ExprCache.Load(expr); ok {
		jm = cached.(*jmespath.JMESPath)
	} else {
		compiled, err := jmespath.Compile(expr)
		if err != nil {
			log.Printf("[Migration-SDK] Failed to compile JMESPath expression '%s': %v", expr, err)
			return false
		}
		jm = compiled
		ExprCache.Store(expr, jm)
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
	return rand.Intn(100) < percentage
}

func matchList(rawValue string, params map[string]interface{}) bool {
	keyList := struct {
		Key  string   `json:"key"`
		List []string `json:"list"`
	}{}
	if err := json.Unmarshal([]byte(rawValue), &keyList); err != nil {
		return false
	}
	paramVal, ok := params[keyList.Key]
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
	for _, item := range keyList.List {
		if item == strVal {
			return true
		}
	}
	return false
}
