package db

import (
	"context"

	"github.com/xmiete/server/internal/models"
)

// Repository is the storage abstraction for deposits.
// Swap the postgres implementation with any other backend by satisfying this interface.
type Repository interface {
	Create(ctx context.Context, d *models.Deposit) (*models.Deposit, error)
	GetByID(ctx context.Context, id string) (*models.Deposit, error)
	UpdateState(ctx context.Context, id string, newState models.LifecycleState, entry models.HistoryEntry, patch func(*models.Deposit)) (*models.Deposit, error)
}
