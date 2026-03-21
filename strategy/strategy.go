package strategy

import (
	"errors"

	"github.com/HBulgat/migration-sdk-go/enums"
	"github.com/HBulgat/migration-sdk-go/grayscale"
	"github.com/HBulgat/migration-sdk-go/model"
)

// Strategy 定义阶段流转协议
type Strategy interface {
	Execute(oldFunc TargetFunc, newFunc TargetFunc, fallbackFunc FallbackFunc,
		paramHandler ParamHandler, postProcessor PostProcessor, migrationKey string, traceId string,
		status enums.MigrationTaskStatus, rules []grayscale.GrayRule, args ...interface{}) (interface{}, error)
}

// TargetFunc 业务目标函数签名
type TargetFunc func(args ...interface{}) (interface{}, error)

// FallbackFunc 业务降级函数签名
type FallbackFunc func(err error, args ...interface{}) (interface{}, error)

// ParamHandler 灰度参数组装器
type ParamHandler func(args ...interface{}) map[string]interface{}

// PostProcessor 数据后置处理函数，在计算 Diff 前对新旧结果进行清洗与平齐
type PostProcessor func(oldRes interface{}, newRes interface{}) (processedOld interface{}, processedNew interface{})


// Reporter 接口剥离（简写直接引用）
type Reporter interface {
	Report(req *model.DiffReportRequest)
}

// Factory 策略工厂
type Factory struct {
	GrayMatcher grayscale.Matcher
	Reporter    Reporter
}

// GetStrategy 根据状态机枚举获取流转策略实现
func (f *Factory) GetStrategy(status enums.MigrationTaskStatus) (Strategy, error) {
	switch status {
	case enums.Old:
		return &OldOnlyStrategy{}, nil
	case enums.ValidationGray:
		return &ValidationGrayStrategy{Matcher: f.GrayMatcher, Reporter: f.Reporter}, nil
	case enums.ValidationAll:
		return &ValidationAllStrategy{Reporter: f.Reporter}, nil
	case enums.GoLiveGray:
		return &GoLiveGrayStrategy{Matcher: f.GrayMatcher, Reporter: f.Reporter}, nil
	case enums.GoLiveAll:
		return &GoLiveAllStrategy{Reporter: f.Reporter}, nil
	case enums.DecommissioningGray:
		return &DecommissioningGrayStrategy{Matcher: f.GrayMatcher, Reporter: f.Reporter}, nil
	case enums.DecommissioningAll:
		return &DecommissioningAllStrategy{}, nil
	default:
		return nil, errors.New("unsupported migration status")
	}
}
