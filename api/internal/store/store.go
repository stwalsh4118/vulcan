package store

import (
	"context"

	"github.com/seantiz/vulcan/internal/model"
)

// Store defines the persistence operations for workloads.
type Store interface {
	CreateWorkload(ctx context.Context, w *model.Workload) error
	GetWorkload(ctx context.Context, id string) (*model.Workload, error)
	ListWorkloads(ctx context.Context, limit, offset int) ([]*model.Workload, int, error)
	UpdateWorkloadStatus(ctx context.Context, id, status string) error
	Close() error
}
