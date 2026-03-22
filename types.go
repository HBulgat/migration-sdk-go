package migration

import (
	"context"

	"github.com/HBulgat/migration-sdk-go/constdef"
	"github.com/HBulgat/migration-sdk-go/gray"
)

type Context struct {
	context.Context
	Client          *Client
	OldFunc         Function
	NewFunc         Function
	FallbackFunc    Function
	ParamHandler    ParamHandler
	Param           map[string]interface{}
	PostProcessor   PostProcessor
	MigrationKey    string
	MigrationStatus constdef.MigrationTaskStatus
	GrayRules       []gray.GrayRule
	Args            []interface{}
	HitGray         bool
}

// Function 业务目标函数签名
type Function func(args ...interface{}) (interface{}, error)

// ParamHandler 灰度参数组装器
type ParamHandler func(args ...interface{}) map[string]interface{}

// PostProcessor 数据后置处理函数，在计算 Diff 前对新旧结果进行清洗与平齐
type PostProcessor func(oldRes interface{}, newRes interface{}) (processedOld interface{}, processedNew interface{})

type executeFunction func(ctx *Context) (interface{}, error)

func invokeOldSync(ctx *Context) (interface{}, error) {
	res, err, _ := InvokeSafely(ctx.OldFunc, ctx.Args...)
	return res, err
}

func invokeNewSync(ctx *Context) (interface{}, error) {
	res, err, _ := InvokeSafely(ctx.NewFunc, ctx.Args...)
	return res, err
}

var executeFunctionMap = map[constdef.MigrationTaskStatus]executeFunction{
	constdef.MigrationTaskStatusUnknown: func(ctx *Context) (interface{}, error) {
		if ctx.FallbackFunc != nil {
			return ctx.FallbackFunc(ctx.Args...)
		}
		return invokeOldSync(ctx)
	},
	constdef.MigrationTaskStatusOld: invokeOldSync,
	constdef.MigrationTaskStatusValidationGray: func(ctx *Context) (interface{}, error) {
		if ctx.HitGray {
			return InvokeConcurrent(ctx, true)
		}
		return invokeOldSync(ctx)
	},
	constdef.MigrationTaskStatusValidationAll: func(ctx *Context) (interface{}, error) {
		return InvokeConcurrent(ctx, true)
	},
	constdef.MigrationTaskStatusGoLiveGray: func(ctx *Context) (interface{}, error) {
		if ctx.HitGray {
			return invokeNewSync(ctx)
		}
		return InvokeConcurrent(ctx, true)
	},
	constdef.MigrationTaskStatusGoLiveAll: func(ctx *Context) (interface{}, error) {
		return InvokeConcurrent(ctx, false)
	},
	constdef.MigrationTaskStatusDecommissioningGray: func(ctx *Context) (interface{}, error) {
		if ctx.HitGray {
			return invokeNewSync(ctx)
		}
		return InvokeConcurrent(ctx, false)
	},
	constdef.MigrationTaskStatusDecommissioningAll: invokeNewSync,
}
