package model

// StatusResponse 针对 /api/internal/sdk/migration_task/query 接口的子结构
type StatusResponse struct {
	TargetStatus int `json:"status"`
}
