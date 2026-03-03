package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/HBulgat/migration-sdk-go"
)

// ========= 模拟业务侧代码 =========

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// targetOld 模拟旧的服务调用
func targetOld(args ...interface{}) (interface{}, error) {
	// 耗时模拟
	time.Sleep(50 * time.Millisecond)
	id := args[0].(string)
	return &User{ID: id, Name: "OldUser", Age: 20}, nil
}

// targetNew 模拟新的服务调用
func targetNew(args ...interface{}) (interface{}, error) {
	time.Sleep(20 * time.Millisecond)
	id := args[0].(string)
	return &User{ID: id, Name: "NewUser", Age: 20}, nil
}

// targetFallback 模拟降级逻辑
func targetFallback(err error, args ...interface{}) (interface{}, error) {
	fmt.Printf("[Fallback] triggered due to error: %v\n", err)
	return nil, errors.New("fallback executed")
}

// userParamHandler 针对此接口抽取灰度参数的规则
func userParamHandler(args ...interface{}) map[string]interface{} {
	return map[string]interface{}{
		"userId": args[0].(string),
		"level":  args[1].(int),
	}
}

func main() {
	// 1. 初始化配置 (实际项目中 AdminUrl/DiffUrl 指向真实地址或走配置中心读取)
	// 由于这只是 Example，我们用占位 URL
	config := &migration.Config{
		MigrationKey:   "user-getUser-api",
		AdminUrl:       "http://localhost:8080",
		DiffServiceUrl: "http://localhost:8081",
	}

	// 2. 构造客户端
	// 此处 NewClient 内部会调用 Provider(API) 拉取路由状态，若失败则降级到 Old(1)
	client := migration.NewClient(config)

	// 3. 得到包装后的执行实例
	executeFn := client.Wrap(targetOld, targetNew, targetFallback, userParamHandler)

	// 4. 业务侧发起调用 (参数：userId="1001", level=5)
	fmt.Println("--- Start Executing ---")
	res, err := executeFn.Execute("1001", 5)

	if err != nil {
		fmt.Printf("Execution Error: %v\n", err)
	} else {
		// 类型断言拿真实结果
		usr := res.(*User)
		fmt.Printf("Execution Success, User Name: %s\n", usr.Name)
	}

	// 停滞程序以确保异步的 Diff 投递协程能有时间打印 Log
	time.Sleep(2 * time.Second)
	fmt.Println("--- Example finished ---")
}
