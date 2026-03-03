package grayscale

import (
	"testing"
)

func TestDefaultMatcher_Percentage(t *testing.T) {
	matcher := &DefaultMatcher{
		Rules: []GrayRule{
			{
				RuleType:  "PERCENTAGE",
				RuleValue: "100",
				Enable:    true,
			},
		},
	}

	params := map[string]interface{}{}
	if !matcher.Match(params) {
		t.Error("Expected 100% to match")
	}

	matcher.Rules[0].RuleValue = "0"
	if matcher.Match(params) {
		t.Error("Expected 0% to not match")
	}
}

func TestDefaultMatcher_Whitelist(t *testing.T) {
	matcher := &DefaultMatcher{
		Rules: []GrayRule{
			{
				RuleType:  "WHITELIST",
				RuleValue: `["1001", "1002"]`,
				Enable:    true,
			},
		},
	}

	paramsMatch := map[string]interface{}{"userId": "1001"}
	if !matcher.Match(paramsMatch) {
		t.Error("Expected userId 1001 to match whitelist")
	}

	paramsNoMatch := map[string]interface{}{"userId": "1003"}
	if matcher.Match(paramsNoMatch) {
		t.Error("Expected userId 1003 to not match whitelist")
	}
}

func TestDefaultMatcher_Blacklist(t *testing.T) {
	matcher := &DefaultMatcher{
		Rules: []GrayRule{
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

	// Blacklist should immediately return false, even if percentage would match next
	paramsBlacklisted := map[string]interface{}{"userId": "1001"}
	if matcher.Match(paramsBlacklisted) {
		t.Error("Expected userId 1001 to be blacklisted and NOT match")
	}

	// Not in blacklist, should fall through to percentage
	paramsAllowed := map[string]interface{}{"userId": "1002"}
	if !matcher.Match(paramsAllowed) {
		t.Error("Expected userId 1002 to match through percentage rule")
	}
}

func TestDefaultMatcher_DisabledRule(t *testing.T) {
	matcher := &DefaultMatcher{
		Rules: []GrayRule{
			{
				RuleType:  "PERCENTAGE",
				RuleValue: "100",
				Enable:    false, // Rule disabled
			},
		},
	}

	params := map[string]interface{}{}
	if matcher.Match(params) {
		t.Error("Expected disabled rule to not match")
	}
}
