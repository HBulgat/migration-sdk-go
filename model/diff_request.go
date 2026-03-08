package model

// DiffReportRequest 提交给 Diff 服务的请求对象
type DiffReportRequest struct {
	MigrationKey        string `json:"migration_key"`
	TraceId             string `json:"trace_id"`
	OldJson             string `json:"old_json"`
	NewJson             string `json:"new_json"`
	OldCostTimeMs       int    `json:"old_cost_time_ms"`
	NewCostTimeMs       int    `json:"new_cost_time_ms"`
	GrayscaleParam      string `json:"grayscale_param"`
	OldSuccess          bool   `json:"old_success"`
	NewSuccess          bool   `json:"new_success"`
	OldErrorMessage     string `json:"old_error_message"`
	NewErrorMessage     string `json:"new_error_message"`
	OldRequestParams    string `json:"old_request_params"`
	NewRequestParams    string `json:"new_request_params"`
	MigrationTaskStatus int    `json:"migration_status"`
	GrayscaleRules      string `json:"grayscale_rules"`
	GrayscaleHit        bool   `json:"grayscale_hit"`
	FallbackTriggered   bool   `json:"fallback_triggered"`
}
