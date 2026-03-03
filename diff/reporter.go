package diff

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Reporter struct {
	DiffServiceUrl string
	Client         *http.Client
	requestChan    chan *DiffReportRequest
}

type DiffReportRequest struct {
	MigrationKey string `json:"migration_key"`
	NewJson      string `json:"new_json"`
	OldJson      string `json:"old_json"`
}

func NewReporter(diffServiceUrl string) *Reporter {
	r := &Reporter{
		DiffServiceUrl: diffServiceUrl,
		Client: &http.Client{
			Timeout: 2 * time.Second,
		},
		requestChan: make(chan *DiffReportRequest, 1000), // 缓冲队列，防止阻塞主业务线程
	}

	// 启动常驻异步消费协程
	go r.startWorker()
	return r
}

func (r *Reporter) startWorker() {
	for req := range r.requestChan {
		r.send(req)
	}
}

func (r *Reporter) Report(migrationKey string, oldRes interface{}, newRes interface{}) {
	// 如果某个结果有 Err，应特别处理或跳过比对（此处简写直接序列化）
	oldBytes, _ := json.Marshal(oldRes)
	newBytes, _ := json.Marshal(newRes)

	req := &DiffReportRequest{
		MigrationKey: migrationKey,
		OldJson:      string(oldBytes),
		NewJson:      string(newBytes),
	}

	// 尝试投递。如果队列满则丢弃，保证不阻塞主业务
	select {
	case r.requestChan <- req:
	default:
		log.Printf("[Migration-SDK] Diff Report Queue is full, dropping payload for migrationKey: %s", migrationKey)
	}
}

func (r *Reporter) send(req *DiffReportRequest) {
	payload, _ := json.Marshal(req)
	resp, err := r.Client.Post(r.DiffServiceUrl+"/api/v1/diff", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("[Migration-SDK] Failed to send diff report: %v", err)
		return
	}
	defer resp.Body.Close()
}
