package diff

import (
	"github.com/HBulgat/migration-sdk-go/model"
)

// DiffReporter 接口剥离（简写直接引用）
type DiffReporter interface {
	Report(req *model.DiffReportRequest)
}
