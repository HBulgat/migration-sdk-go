package migration

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/HBulgat/migration-sdk-go/model"
)

type invokeResult struct {
	res    interface{}
	err    error
	costMs int64
}

// InvokeSafely 安全地调用指定方法，并记录执行的耗时。
func InvokeSafely(fn Function, args ...interface{}) (res interface{}, err error, costMs int64) {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Migration-SDK] Panic in func: %v", r)
			err = fmt.Errorf("panic: %v", r)
		}
		costMs = time.Since(start).Milliseconds()
	}()
	res, err = fn(args...)
	return
}

// InvokeConcurrent 抽象并发调用主从流程
// isMainOld = true 时，OldFunc为主流程同步执行并作为返回值；NewFunc为从流程异步执行并用于比对。
// isMainOld = false 时，NewFunc为主流程，OldFunc为从流程。
func InvokeConcurrent(ctx *Context, isMainOld bool) (interface{}, error) {
	var masterFunc, slaveFunc Function
	if isMainOld {
		masterFunc = ctx.OldFunc
		slaveFunc = ctx.NewFunc
	} else {
		masterFunc = ctx.NewFunc
		slaveFunc = ctx.OldFunc
	}

	masterResultChan := make(chan invokeResult, 1)

	// 从流程：后台异步执行
	go func() {
		slaveRes, slaveErr, slaveCost := InvokeSafely(slaveFunc, ctx.Args...)

		var masterRes interface{}
		var masterErr error
		var masterCost int64
		timeout := time.After(5 * time.Second) // 带超时保护等待主流程
		select {
		case master := <-masterResultChan:
			masterRes, masterErr, masterCost = master.res, master.err, master.costMs
		case <-timeout:
			log.Printf("[Migration-SDK] Background diff task timeout waiting for master interface: migrationKey=%s", ctx.MigrationKey)
			masterErr = fmt.Errorf("background task timeout waiting for master result")
		}

		// 根据标志位恢复新旧结果供发送 Diff
		if isMainOld {
			ReportDiff(ctx, masterRes, masterErr, masterCost, slaveRes, slaveErr, slaveCost)
		} else {
			ReportDiff(ctx, slaveRes, slaveErr, slaveCost, masterRes, masterErr, masterCost)
		}
	}()

	// 主流程：主线程同步执行
	masterRes, masterErr, masterCost := InvokeSafely(masterFunc, ctx.Args...)

	// 将主流程结果传递给后台协程
	masterResultChan <- invokeResult{masterRes, masterErr, masterCost}

	return masterRes, masterErr
}

// ReportDiff 发送比对结果到 Diff 服务
func ReportDiff(ctx *Context, oldRes interface{}, oldErr error, oldCost int64, newRes interface{}, newErr error, newCost int64) {
	if ctx.Client == nil || ctx.Client.diffReporter == nil || *ctx.Client.diffReporter == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Migration-SDK] Send diff async panic, migrationKey=%s, error=%v", ctx.MigrationKey, r)
		}
	}()

	var traceId string // 目前 SDK 暂不支持获取外部的 trace_id，按原样留空

	processedOld := oldRes
	processedNew := newRes
	if ctx.PostProcessor != nil {
		processedOld, processedNew = ctx.PostProcessor(oldRes, newRes)
	}

	oldSuccess := oldErr == nil
	newSuccess := newErr == nil

	safeJson := func(obj interface{}, success bool) string {
		if !success || obj == nil {
			return ""
		}
		b, _ := json.Marshal(obj)
		return string(b)
	}

	oldErrMsg, newErrMsg := "", ""
	if oldErr != nil {
		oldErrMsg = oldErr.Error()
	}
	if newErr != nil {
		newErrMsg = newErr.Error()
	}

	req := &model.DiffReportRequest{
		MigrationKey:        ctx.MigrationKey,
		TraceId:             traceId,
		OldJson:             safeJson(processedOld, oldSuccess),
		NewJson:             safeJson(processedNew, newSuccess),
		OldCostTimeMs:       int(oldCost),
		NewCostTimeMs:       int(newCost),
		GrayscaleParam:      safeJson(ctx.Param, true),
		OldSuccess:          oldSuccess,
		NewSuccess:          newSuccess,
		OldErrorMessage:     oldErrMsg,
		NewErrorMessage:     newErrMsg,
		OldRequestParams:    safeJson(ctx.Args, true),
		NewRequestParams:    safeJson(ctx.Args, true),
		MigrationTaskStatus: int(ctx.MigrationStatus),
		GrayscaleRules:      safeJson(ctx.GrayRules, true),
		GrayscaleHit:        ctx.HitGray,
		FallbackTriggered:   false,
	}

	(*ctx.Client.diffReporter).Report(req)
}

// ExecuteFallbackAfterFailed 当主路由接口失败时，处理降级逻辑
func ExecuteFallbackAfterFailed(ctx *Context, err error) (interface{}, error) {
	if ctx.FallbackFunc != nil {
		return ctx.FallbackFunc(ctx.Args...)
	}
	return nil, err
}
