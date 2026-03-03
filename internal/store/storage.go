package store

import (
	"context"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
)

type TargetStorage interface {
	CreateTarget(ctx context.Context, target models.Target) (models.Target, error)
	GetTarget(ctx context.Context, id string) (models.Target, error)
	ListTargets(ctx context.Context) ([]models.Target, error)
	UpdateTarget(ctx context.Context, target models.Target) (models.Target, error)
	DeleteTarget(ctx context.Context, id string) error
	Close() error
}

func NewTargetStore(storageType string) TargetStorage {
	switch storageType {
	case "sqlite":
		return newSQLiteTargetStore()
	default:
		return newMemoryTargetStore()
	}
}
