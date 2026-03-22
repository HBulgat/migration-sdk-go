package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/HBulgat/migration-sdk-go/constdef"
	"github.com/HBulgat/migration-sdk-go/gray"
	"github.com/HBulgat/migration-sdk-go/model"
)

// HttpConfigClient 基于 Admin Http API 拉取配置的默认实现
type HttpConfigClient struct {
	BaseURL       string
	InternalToken string // 用于访问内部接口的鉴权 Token
	Client        *http.Client
}

func NewHttpConfigClient(address, internalToken string, timeoutSeconds time.Duration) *HttpConfigClient {
	return &HttpConfigClient{
		BaseURL:       address,
		InternalToken: internalToken,
		Client: &http.Client{
			Timeout: timeoutSeconds * time.Second,
		},
	}
}

type GetStatusRequest struct {
	MigrationKey string `json:"migration_key"`
}

func (c HttpConfigClient) GetQueryMigrationTaskURL() string {
	return fmt.Sprintf("%s/api/internal/sdk/migration_task/query", c.BaseURL)
}

func (c HttpConfigClient) GetQueryGrayRulesURL(migrationKey string) string {
	return fmt.Sprintf("%s/api/internal/sdk/gray_rule/list?migration_key=%s", c.BaseURL, migrationKey)
}

func (c HttpConfigClient) GetStatus(migrationKey string) (constdef.MigrationTaskStatus, error) {
	url := c.GetQueryMigrationTaskURL()
	payload := &GetStatusRequest{
		MigrationKey: migrationKey,
	}
	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return 0, err
	}
	resp, err := c.executeRequest(req)
	if err != nil {
		return 0, err
	}
	var result model.Result[model.StatusResponse]
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, err
	}
	if result.Code != 0 && result.Code != 200 {
		return 0, fmt.Errorf("failed to get status, code: %d, message: %s", result.Code, result.Message)
	}
	status := result.Data.TargetStatus
	return constdef.MigrationTaskStatus(status), nil
}

func (c HttpConfigClient) GetGrayRules(migrationKey string) ([]gray.Rule, error) {
	url := c.GetQueryGrayRulesURL(migrationKey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.executeRequest(req)
	if err != nil {
		return nil, err
	}
	var result model.Result[[]gray.Rule]
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	if result.Code != 0 && result.Code != 200 {
		return nil, fmt.Errorf("failed to get gray rules, code: %d, message: %s", result.Code, result.Message)
	}
	return result.Data, nil
}

func (c HttpConfigClient) executeRequest(req *http.Request) ([]byte, error) {
	if req.Method != http.MethodGet && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.InternalToken != "" {
		req.Header.Set("X-Internal-Token", c.InternalToken)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
