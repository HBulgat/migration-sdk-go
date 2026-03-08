package grayscale

import (
	"testing"
)

type mockRuleProvider struct {
	rules []GrayRule
}

func (m *mockRuleProvider) GetRules(migrationKey string) []GrayRule {
	return m.rules
}

func TestDefaultMatcher_Percentage(t *testing.T) {
	provider := &mockRuleProvider{
		rules: []GrayRule{
			{
				RuleType:  "PERCENTAGE",
				RuleValue: "100",
				Enable:    true,
			},
		},
	}
	matcher := &DefaultMatcher{RuleProvider: provider}

	params := map[string]interface{}{}
	if !matcher.Match("test_key", params) {
		t.Error("Expected 100% to match")
	}

	provider.rules[0].RuleValue = "0"
	if matcher.Match("test_key", params) {
		t.Error("Expected 0% to not match")
	}
}

func TestDefaultMatcher_Whitelist(t *testing.T) {
	provider := &mockRuleProvider{
		rules: []GrayRule{
			{
				RuleType:  "WHITELIST",
				RuleValue: `["1001", "1002"]`,
				Enable:    true,
			},
		},
	}
	matcher := &DefaultMatcher{RuleProvider: provider}

	paramsMatch := map[string]interface{}{"userId": "1001"}
	if !matcher.Match("test_key", paramsMatch) {
		t.Error("Expected userId 1001 to match whitelist")
	}

	paramsNoMatch := map[string]interface{}{"userId": "1003"}
	if matcher.Match("test_key", paramsNoMatch) {
		t.Error("Expected userId 1003 to not match whitelist")
	}
}

func TestDefaultMatcher_Blacklist(t *testing.T) {
	provider := &mockRuleProvider{
		rules: []GrayRule{
			{
				RuleType:  "BLACKLIST",
				RuleValue: `["1001"]`,
				Enable:    true,
			},
			{
				RuleType:  "PERCENTAGE",
				RuleValue: "100",
				Enable:    true,
			},
		},
	}
	matcher := &DefaultMatcher{RuleProvider: provider}

	// Blacklist should immediately return false, even if percentage would match next
	paramsBlacklisted := map[string]interface{}{"userId": "1001"}
	if matcher.Match("test_key", paramsBlacklisted) {
		t.Error("Expected userId 1001 to be blacklisted and NOT match")
	}

	// Not in blacklist, should fall through to percentage
	paramsAllowed := map[string]interface{}{"userId": "1002"}
	if !matcher.Match("test_key", paramsAllowed) {
		t.Error("Expected userId 1002 to match through percentage rule")
	}
}

func TestDefaultMatcher_DisabledRule(t *testing.T) {
	provider := &mockRuleProvider{
		rules: []GrayRule{
			{
				RuleType:  "PERCENTAGE",
				RuleValue: "100",
				Enable:    false, // Rule disabled
			},
		},
	}
	matcher := &DefaultMatcher{RuleProvider: provider}

	params := map[string]interface{}{}
	if matcher.Match("test_key", params) {
		t.Error("Expected disabled rule to not match")
	}
}
