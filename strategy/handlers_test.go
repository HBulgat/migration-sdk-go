package strategy

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HBulgat/migration-sdk-go/enums"
	"github.com/HBulgat/migration-sdk-go/model"
)

// MockTargetFunc Helper target function for testing
func MockTargetFunc(result interface{}, err error, delay time.Duration) TargetFunc {
	return func(args ...interface{}) (interface{}, error) {
		if delay > 0 {
			time.Sleep(delay)
		}
		return result, err
	}
}

func MockFallbackFunc() FallbackFunc {
	return func(err error, args ...interface{}) (interface{}, error) {
		return "fallback_result", errors.New("fallback executed")
	}
}

func MockParamHandler() ParamHandler {
	return func(args ...interface{}) map[string]interface{} {
		return map[string]interface{}{}
	}
}

// MockMatcher Mock grayscale matcher
type MockMatcher struct {
	ShouldMatch bool
}

func (m *MockMatcher) Match(migrationKey string, params map[string]interface{}) bool {
	return m.ShouldMatch
}

// MockReporter Mock report collector
type MockReporter struct {
	sync.Mutex
	Reports []string
}

func (r *MockReporter) Report(req *model.DiffReportRequest) {
	r.Lock()
	defer r.Unlock()

	migrationKey := req.MigrationKey
	oldStr := "nil"
	if req.OldJson != "null" && req.OldJson != "" {
		oldStr = strings.Trim(req.OldJson, "\"")
	}

	newStr := "nil"
	if req.NewJson != "null" && req.NewJson != "" {
		newStr = strings.Trim(req.NewJson, "\"")
	}

	r.Reports = append(r.Reports, migrationKey+"|"+oldStr+"|"+newStr)
}

func TestOldOnlyStrategy(t *testing.T) {
	factory := &Factory{}
	s, _ := factory.GetStrategy(enums.Old)

	oldFunc := MockTargetFunc("old_result", nil, 0)
	newFunc := MockTargetFunc("new_result", nil, 0)

	res, err := s.Execute(oldFunc, newFunc, nil, MockParamHandler(), nil, "key", "trace_id_123", enums.Old, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if res != "old_result" {
		t.Errorf("Expected old_result, got %v", res)
	}
}

func TestValidationAllStrategy(t *testing.T) {
	reporter := &MockReporter{}
	factory := &Factory{Reporter: reporter}
	s, _ := factory.GetStrategy(enums.ValidationAll)

	oldFunc := MockTargetFunc("old_result", nil, 10*time.Millisecond)
	newFunc := MockTargetFunc("new_result", nil, 5*time.Millisecond)

	res, err := s.Execute(oldFunc, newFunc, nil, MockParamHandler(), nil, "key_test_val_all", "trace_id_123", enums.ValidationAll, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if res != "old_result" {
		t.Errorf("Expected old_result, got %v. Validation should return old.", res)
	}

	// Wait for async diff thread
	time.Sleep(20 * time.Millisecond)

	reporter.Lock()
	defer reporter.Unlock()
	if len(reporter.Reports) != 1 {
		t.Fatalf("Expected 1 diff report, got %d", len(reporter.Reports))
	}
	if !strings.Contains(reporter.Reports[0], "key_test_val_all|old_result|new_result") {
		t.Errorf("Report content mismatch: %s", reporter.Reports[0])
	}
}

func TestGoLiveGrayStrategy_Hit(t *testing.T) {
	reporter := &MockReporter{}
	// Set Matcher to return true
	factory := &Factory{Reporter: reporter, GrayMatcher: &MockMatcher{ShouldMatch: true}}
	s, _ := factory.GetStrategy(enums.GoLiveGray)

	oldFunc := MockTargetFunc("old_result", nil, 10*time.Millisecond)
	newFunc := MockTargetFunc("new_result", nil, 5*time.Millisecond)

	res, err := s.Execute(oldFunc, newFunc, nil, MockParamHandler(), nil, "key_hit", "trace_id_123", enums.GoLiveGray, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if res != "new_result" { // Hit means we return new result
		t.Errorf("Expected new_result, got %v", res)
	}

	// wait for potential async diff routing
	time.Sleep(20 * time.Millisecond)

	reporter.Lock()
	defer reporter.Unlock()
	if len(reporter.Reports) != 0 {
		t.Fatalf("Expected 0 diff reports since hit Gray in GoLiveGray, got %d", len(reporter.Reports))
	}
}

func TestGoLiveGrayStrategy_Miss(t *testing.T) {
	reporter := &MockReporter{}
	// Set Matcher to return false
	factory := &Factory{Reporter: reporter, GrayMatcher: &MockMatcher{ShouldMatch: false}}
	s, _ := factory.GetStrategy(enums.GoLiveGray)

	oldFunc := MockTargetFunc("old_result", nil, 10*time.Millisecond)
	newFunc := MockTargetFunc("new_result", nil, 5*time.Millisecond)

	res, err := s.Execute(oldFunc, newFunc, nil, MockParamHandler(), nil, "key_miss", "trace_id_123", enums.GoLiveGray, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if res != "old_result" { // Miss means we return old result
		t.Errorf("Expected old_result, got %v", res)
	}
}

func TestFallback(t *testing.T) {
	factory := &Factory{}
	s, _ := factory.GetStrategy(enums.Old) // Old strategy just calls invokeWithFallback

	// Old func errors, fallback kicks in
	oldFuncWithError := MockTargetFunc(nil, errors.New("original error"), 0)

	res, err := s.Execute(oldFuncWithError, nil, MockFallbackFunc(), MockParamHandler(), nil, "key", "trace_id_123", enums.Old, nil)

	if err == nil || err.Error() != "fallback executed" {
		t.Errorf("Expected fallback error, got %v", err)
	}

	if res != "fallback_result" {
		t.Errorf("Expected fallback_result, got %v", res)
	}
}
