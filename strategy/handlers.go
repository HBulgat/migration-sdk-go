package strategy

import (
	"log"

	"github.com/HBulgat/migration-sdk-go/grayscale"
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
	res interface{}
	err error
} {
	ch := make(chan struct {
		res interface{}
		err error
	}, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Migration-SDK] Panic recovered in async invoke: %v", r)
			}
		}()
		res, err := invokeWithFallback(target, fallback, args...)
		ch <- struct {
			res interface{}
			err error
		}{res, err}
	}()
	return ch
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
	hit := s.Matcher.Match(params)

	newCh := asyncInvoke(newFunc, fallbackFunc, args...)
	oldRes, oldErr := invokeWithFallback(oldFunc, fallbackFunc, args...)

	go func() {
		newRow := <-newCh
		if hit && s.Reporter != nil {
			s.Reporter.Report(migrationKey, oldRes, newRow.res)
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
	newCh := asyncInvoke(newFunc, fallbackFunc, args...)
	oldRes, oldErr := invokeWithFallback(oldFunc, fallbackFunc, args...)
	go func() {
		newRow := <-newCh
		if s.Reporter != nil {
			s.Reporter.Report(migrationKey, oldRes, newRow.res)
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
	hit := s.Matcher.Match(params)

	if hit {
		oldCh := asyncInvoke(oldFunc, fallbackFunc, args...)
		newRes, newErr := invokeWithFallback(newFunc, fallbackFunc, args...)
		go func() {
			oldRow := <-oldCh
			if s.Reporter != nil {
				s.Reporter.Report(migrationKey, oldRow.res, newRes)
			}
		}()
		return newRes, newErr
	}

	newCh := asyncInvoke(newFunc, fallbackFunc, args...)
	oldRes, oldErr := invokeWithFallback(oldFunc, fallbackFunc, args...)
	go func() {
		newRow := <-newCh
		if s.Reporter != nil {
			s.Reporter.Report(migrationKey, oldRes, newRow.res)
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
	oldCh := asyncInvoke(oldFunc, fallbackFunc, args...)
	newRes, newErr := invokeWithFallback(newFunc, fallbackFunc, args...)
	go func() {
		oldRow := <-oldCh
		if s.Reporter != nil {
			s.Reporter.Report(migrationKey, oldRow.res, newRes)
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
	hit := s.Matcher.Match(params)

	if hit {
		return invokeWithFallback(newFunc, fallbackFunc, args...)
	}

	oldCh := asyncInvoke(oldFunc, fallbackFunc, args...)
	newRes, newErr := invokeWithFallback(newFunc, fallbackFunc, args...)
	go func() {
		oldRow := <-oldCh
		if s.Reporter != nil {
			s.Reporter.Report(migrationKey, oldRow.res, newRes)
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
