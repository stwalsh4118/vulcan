package store

import (
	"context"
	"errors"

	"github.com/seantiz/vulcan/internal/model"
)

// ErrInvalidTransition is returned when a workload status transition is not allowed.
var ErrInvalidTransition = errors.New("invalid status transition")

// WorkloadStats holds aggregate execution statistics.
type WorkloadStats struct {
	Total            int            `json:"total"`
	CountByStatus    map[string]int `json:"count_by_status"`
	CountByIsolation map[string]int `json:"count_by_isolation"`
	AvgDurationMS    float64        `json:"avg_duration_ms"`
}

// Store defines the persistence operations for workloads.
type Store interface {
	CreateWorkload(ctx context.Context, w *model.Workload) error
	GetWorkload(ctx context.Context, id string) (*model.Workload, error)
	ListWorkloads(ctx context.Context, limit, offset int) ([]*model.Workload, int, error)
	UpdateWorkloadStatus(ctx context.Context, id, status string) error
	UpdateWorkload(ctx context.Context, w *model.Workload) error
	GetWorkloadStats(ctx context.Context) (*WorkloadStats, error)
	InsertLogLine(ctx context.Context, workloadID string, seq int, line string) error
	GetLogLines(ctx context.Context, workloadID string) ([]model.LogLine, error)
	Close() error
}
