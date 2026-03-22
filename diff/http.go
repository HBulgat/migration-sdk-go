package diff

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/HBulgat/migration-sdk-go/model"
)

type HttpDiffReporter struct {
	Address       string
	InternalToken string
	Client        *http.Client
	requestChan   chan *model.DiffReportRequest
}

func NewHttpDiffReporter(address, internalToken string, timeoutSeconds time.Duration) *HttpDiffReporter {
	r := &HttpDiffReporter{
		Address:       address,
		InternalToken: internalToken,
		Client: &http.Client{
			Timeout: timeoutSeconds * time.Second,
		},
		requestChan: make(chan *model.DiffReportRequest, 1000), // 缓冲队列，防止阻塞主业务线程
	}
	// 启动常驻异步消费协程
	go r.startWorker()
	return r
}

func (r *HttpDiffReporter) startWorker() {
	for req := range r.requestChan {
		r.send(req)
	}
}

func (r *HttpDiffReporter) Report(req *model.DiffReportRequest) {
	// 尝试投递。如果队列满则丢弃，保证不阻塞主业务
	select {
	case r.requestChan <- req:
	default:
		log.Printf("[Migration-SDK] Diff Report Queue is full, dropping payload for migrationKey: %s", req.MigrationKey)
	}
}

func (r *HttpDiffReporter) send(req *model.DiffReportRequest) {
	payload, _ := json.Marshal(req)
	resp, err := r.Client.Post(r.Address+"/api/v1/diff", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("[Migration-SDK] Failed to send diff report: %v", err)
		return
	}
	defer resp.Body.Close()
}
