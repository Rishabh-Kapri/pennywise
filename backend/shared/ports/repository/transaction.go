package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type TransactionRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID, filter *model.TransactionFilter) ([]model.Transaction, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Transaction, error)
	GetAllNormalized(
		ctx context.Context,
		budgetId uuid.UUID,
		filter *model.TransactionFilter,
	) (model.PaginatedResponse[model.Transaction], error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, txn model.Transaction) error
	UpdateLocation(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, lat *float64, lng *float64, name *string, source *model.LocationSource) error
	UpdateStatus(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, status model.TransactionStatus) error
	Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error)
	CreateDeduped(ctx context.Context, txn model.Transaction) (*model.Transaction, bool, error)
	// LockForDedupe holds a lock on the account and amount until the caller's
	// transaction ends, so two messages for one payment can't both create it.
	LockForDedupe(ctx context.Context, budgetID, accountID uuid.UUID, amount float64) error
	// FindByDedupeHash returns the transaction with this hash, or nil if none.
	FindByDedupeHash(ctx context.Context, budgetID uuid.UUID, hash string) (*model.Transaction, error)
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}
