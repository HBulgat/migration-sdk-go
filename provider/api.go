package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/HBulgat/migration-sdk-go/enums"
	"github.com/HBulgat/migration-sdk-go/grayscale"
	"github.com/HBulgat/migration-sdk-go/model"
)

// APIProvider 基于 Admin API 拉取配置的默认实现
type APIProvider struct {
	AdminUrl      string // e.g. http://localhost:8080
	InternalToken string // 用于访问内部接口的鉴权 Token
	Client        *http.Client
}

func NewAPIProvider(adminUrl, internalToken string) *APIProvider {
	return &APIProvider{
		AdminUrl:      adminUrl,
		InternalToken: internalToken,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (p *APIProvider) GetStatus(migrationKey string) (enums.MigrationTaskStatus, error) {
	url := fmt.Sprintf("%s/api/internal/sdk/migration_task/query", p.AdminUrl)

	payload := map[string]string{
		"migration_key": migrationKey,
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.InternalToken != "" {
		req.Header.Set("X-Internal-Token", p.InternalToken)
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result model.Result[model.StatusResponse]
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	if result.Code != 0 {
		return 0, fmt.Errorf("failed to get status, code: %d, message: %s", result.Code, result.Message)
	}

	status := result.Data.TargetStatus
	if status == 0 {
		status = int(enums.Old) // Default to Old if missing or empty
	}
	return enums.MigrationTaskStatus(status), nil
}

func (p *APIProvider) GetGrayRules(migrationKey string) ([]grayscale.GrayRule, error) {
	url := fmt.Sprintf("%s/api/internal/sdk/grayscale_rule/list?migration_key=%s", p.AdminUrl, migrationKey)
	rawJson, err := p.fetchRawJson(url)
	if err != nil {
		return nil, err
	}

	var result model.Result[[]grayscale.GrayRule]
	if err := json.Unmarshal([]byte(rawJson), &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("failed to get gray rules, code: %d, message: %s", result.Code, result.Message)
	}

	return result.Data, nil
}

func (p *APIProvider) fetchRawJson(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	if p.InternalToken != "" {
		req.Header.Set("X-Internal-Token", p.InternalToken)
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 返回原始 JSON，供具体策略使用前反序列化
	return string(body), nil
}
