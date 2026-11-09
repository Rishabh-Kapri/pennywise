package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type AccountRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Account, error)
	GetAllSimplified(ctx context.Context, budgetId uuid.UUID) ([]model.AccountSimplified, error)
	GetById(ctx context.Context, budgetId uuid.UUID, accountId uuid.UUID) (*model.Account, error)
	GetBySuffix(ctx context.Context, budgetId uuid.UUID, suffix string) (*model.Account, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Account, error)
	Create(ctx context.Context, account model.Account) (*model.Account, error)
	// Update changes the editable fields (name, bank, suffix, closed); the type is fixed.
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, account model.Account) (*model.Account, error)
	UpdateTransferPayee(ctx context.Context, accountId uuid.UUID, payeeId uuid.UUID) error
}
