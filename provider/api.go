package provider

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// APIProvider 基于 Admin API 拉取配置的默认实现
type APIProvider struct {
	AdminUrl string // e.g. http://localhost:8080
	Client   *http.Client
}

func NewAPIProvider(adminUrl string) *APIProvider {
	return &APIProvider{
		AdminUrl: adminUrl,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (p *APIProvider) GetStatus(migrationKey string) (int, error) {
	// 简化的 API 调用，实际需要对齐 /api/v1/migration_task/list 接口数据结构
	url := fmt.Sprintf("%s/api/v1/migration_task/list?migration_key=%s", p.AdminUrl, migrationKey)
	resp, err := p.Client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				TargetStatus int `json:"status"`
			} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	if result.Code != 0 || len(result.Data.List) == 0 {
		return 0, fmt.Errorf("failed to get status, code: %d", result.Code)
	}

	// 这里取 target_status 仅为演示。通常业务方 SDK 使用单独简化的查询接口或直接基于配置中心
	return result.Data.List[0].TargetStatus, nil
}

func (p *APIProvider) GetGrayRules(migrationKey string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/grayscale_rule/list?migration_key=%s", p.AdminUrl, migrationKey)
	return p.fetchRawJson(url)
}

func (p *APIProvider) GetDiffRules(migrationKey string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/diff_rule/list?migration_key=%s", p.AdminUrl, migrationKey)
	return p.fetchRawJson(url)
}

func (p *APIProvider) fetchRawJson(url string) (string, error) {
	resp, err := p.Client.Get(url)
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
