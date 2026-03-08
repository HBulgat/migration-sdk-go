package strategy

import (
	"encoding/json"
	"log"
	"time"

	"github.com/HBulgat/migration-sdk-go/grayscale"
	"github.com/HBulgat/migration-sdk-go/model"
)

// invokeWithFallback 执行并携带降级处理逻辑
func invokeWithFallback(target TargetFunc, fallback FallbackFunc, args ...interface{}) (interface{}, error) {
	res, err := target(args...)
	if err != nil && fallback != nil {
		return fallback(err, args...)
	}
	return res, err
}

// asyncInvoke 异步执行方法及容错捕获
func asyncInvoke(target TargetFunc, fallback FallbackFunc, args ...interface{}) <-chan struct {
	res      interface{}
	err      error
	costTime int64
} {
	ch := make(chan struct {
		res      interface{}
		err      error
		costTime int64
	}, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Migration-SDK] Panic recovered in async invoke: %v", r)
			}
		}()
		start := time.Now()
		res, err := invokeWithFallback(target, fallback, args...)
		cost := time.Since(start).Milliseconds()
		ch <- struct {
			res      interface{}
			err      error
			costTime int64
		}{res, err, cost}
	}()
	return ch
}

// buildDiffReq 构造提交给 Diff 服务的请求对象
func buildDiffReq(migrationKey string, oldRes interface{}, oldErr error, oldCost int64, newRes interface{}, newErr error, newCost int64, params map[string]interface{}, hit bool, args ...interface{}) *model.DiffReportRequest {
	oldBytes, _ := json.Marshal(oldRes)
	newBytes, _ := json.Marshal(newRes)
	paramBytes, _ := json.Marshal(params)
	argsBytes, _ := json.Marshal(args)

	oldSuccess := oldErr == nil
	newSuccess := newErr == nil
	var oldErrStr, newErrStr string
	if oldErr != nil {
		oldErrStr = oldErr.Error()
	}
	if newErr != nil {
		newErrStr = newErr.Error()
	}

	// Create an anonymous struct matching diff.DiffReportRequest structure purely
	return &model.DiffReportRequest{
		MigrationKey:        migrationKey,
		OldJson:             string(oldBytes),
		NewJson:             string(newBytes),
		OldCostTimeMs:       int(oldCost),
		NewCostTimeMs:       int(newCost),
		GrayscaleParam:      string(paramBytes),
		OldSuccess:          oldSuccess,
		NewSuccess:          newSuccess,
		OldErrorMessage:     oldErrStr,
		NewErrorMessage:     newErrStr,
		OldRequestParams:    string(argsBytes),
		NewRequestParams:    string(argsBytes),
		MigrationTaskStatus: 0, // Not explicitly passed inside simple Strategy, left as 0
		GrayscaleRules:      "",
		GrayscaleHit:        hit,
		FallbackTriggered:   oldErr != nil || newErr != nil,
	}
}

// ========== 具体 7 阶段实现 ==========

// 1. 旧接口
type OldOnlyStrategy struct{}

func (s *OldOnlyStrategy) Execute(oldFunc TargetFunc, newFunc TargetFunc, fallbackFunc FallbackFunc,
	paramHandler ParamHandler, migrationKey string, args ...interface{}) (interface{}, error) {
	return invokeWithFallback(oldFunc, fallbackFunc, args...)
}

// 2. 验证-灰度 (并发执行，仅旧回，发 Diff)
type ValidationGrayStrategy struct {
	Matcher  grayscale.Matcher
	Reporter Reporter
}

func (s *ValidationGrayStrategy) Execute(oldFunc TargetFunc, newFunc TargetFunc, fallbackFunc FallbackFunc,
	paramHandler ParamHandler, migrationKey string, args ...interface{}) (interface{}, error) {
	params := paramHandler(args...)
	hit := s.Matcher.Match(migrationKey, params)

	newCh := asyncInvoke(newFunc, fallbackFunc, args...)
	start := time.Now()
	oldRes, oldErr := invokeWithFallback(oldFunc, fallbackFunc, args...)
	oldCost := time.Since(start).Milliseconds()

	go func() {
		newRow := <-newCh
		if hit && s.Reporter != nil {
			req := buildDiffReq(migrationKey, oldRes, oldErr, oldCost, newRow.res, newRow.err, newRow.costTime, params, hit, args...)
			s.Reporter.Report(req)
		}
	}()
	return oldRes, oldErr
}

// 3. 验证-全开
type ValidationAllStrategy struct {
	Reporter Reporter
}

func (s *ValidationAllStrategy) Execute(oldFunc TargetFunc, newFunc TargetFunc, fallbackFunc FallbackFunc,
	paramHandler ParamHandler, migrationKey string, args ...interface{}) (interface{}, error) {
	params := paramHandler(args...)
	newCh := asyncInvoke(newFunc, fallbackFunc, args...)
	start := time.Now()
	oldRes, oldErr := invokeWithFallback(oldFunc, fallbackFunc, args...)
	oldCost := time.Since(start).Milliseconds()

	go func() {
		newRow := <-newCh
		if s.Reporter != nil {
			req := buildDiffReq(migrationKey, oldRes, oldErr, oldCost, newRow.res, newRow.err, newRow.costTime, params, false, args...)
			s.Reporter.Report(req)
		}
	}()
	return oldRes, oldErr
}

// 4. 上线-灰度
type GoLiveGrayStrategy struct {
	Matcher  grayscale.Matcher
	Reporter Reporter
}

func (s *GoLiveGrayStrategy) Execute(oldFunc TargetFunc, newFunc TargetFunc, fallbackFunc FallbackFunc,
	paramHandler ParamHandler, migrationKey string, args ...interface{}) (interface{}, error) {
	params := paramHandler(args...)
	hit := s.Matcher.Match(migrationKey, params)

	if hit {
		oldCh := asyncInvoke(oldFunc, fallbackFunc, args...)
		start := time.Now()
		newRes, newErr := invokeWithFallback(newFunc, fallbackFunc, args...)
		newCost := time.Since(start).Milliseconds()
		go func() {
			oldRow := <-oldCh
			if s.Reporter != nil {
				req := buildDiffReq(migrationKey, oldRow.res, oldRow.err, oldRow.costTime, newRes, newErr, newCost, params, hit, args...)
				s.Reporter.Report(req)
			}
		}()
		return newRes, newErr
	}

	newCh := asyncInvoke(newFunc, fallbackFunc, args...)
	start := time.Now()
	oldRes, oldErr := invokeWithFallback(oldFunc, fallbackFunc, args...)
	oldCost := time.Since(start).Milliseconds()
	go func() {
		newRow := <-newCh
		if s.Reporter != nil {
			req := buildDiffReq(migrationKey, oldRes, oldErr, oldCost, newRow.res, newRow.err, newRow.costTime, params, hit, args...)
			s.Reporter.Report(req)
		}
	}()
	return oldRes, oldErr
}

// 5. 上线-全开
type GoLiveAllStrategy struct {
	Reporter Reporter
}

func (s *GoLiveAllStrategy) Execute(oldFunc TargetFunc, newFunc TargetFunc, fallbackFunc FallbackFunc,
	paramHandler ParamHandler, migrationKey string, args ...interface{}) (interface{}, error) {
	params := paramHandler(args...)
	oldCh := asyncInvoke(oldFunc, fallbackFunc, args...)
	start := time.Now()
	newRes, newErr := invokeWithFallback(newFunc, fallbackFunc, args...)
	newCost := time.Since(start).Milliseconds()

	go func() {
		oldRow := <-oldCh
		if s.Reporter != nil {
			req := buildDiffReq(migrationKey, oldRow.res, oldRow.err, oldRow.costTime, newRes, newErr, newCost, params, false, args...)
			s.Reporter.Report(req)
		}
	}()
	return newRes, newErr
}

// 6. 停用-灰度
type DecommissioningGrayStrategy struct {
	Matcher  grayscale.Matcher
	Reporter Reporter
}

func (s *DecommissioningGrayStrategy) Execute(oldFunc TargetFunc, newFunc TargetFunc, fallbackFunc FallbackFunc,
	paramHandler ParamHandler, migrationKey string, args ...interface{}) (interface{}, error) {
	params := paramHandler(args...)
	hit := s.Matcher.Match(migrationKey, params)

	if hit {
		return invokeWithFallback(newFunc, fallbackFunc, args...)
	}

	oldCh := asyncInvoke(oldFunc, fallbackFunc, args...)
	start := time.Now()
	newRes, newErr := invokeWithFallback(newFunc, fallbackFunc, args...)
	newCost := time.Since(start).Milliseconds()

	go func() {
		oldRow := <-oldCh
		if s.Reporter != nil {
			req := buildDiffReq(migrationKey, oldRow.res, oldRow.err, oldRow.costTime, newRes, newErr, newCost, params, hit, args...)
			s.Reporter.Report(req)
		}
	}()
	return newRes, newErr
}

// 7. 停用-全开
type DecommissioningAllStrategy struct{}

func (s *DecommissioningAllStrategy) Execute(oldFunc TargetFunc, newFunc TargetFunc, fallbackFunc FallbackFunc,
	paramHandler ParamHandler, migrationKey string, args ...interface{}) (interface{}, error) {
	return invokeWithFallback(newFunc, fallbackFunc, args...)
}
