package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type LoanMetadataRepository interface {
	GetAllByBudgetId(ctx context.Context, budgetId uuid.UUID) ([]model.LoanMetadata, error)
	GetByAccountId(ctx context.Context, accountId uuid.UUID) (*model.LoanMetadata, error)
	Create(ctx context.Context, loan model.LoanMetadata) (*model.LoanMetadata, error)
	Update(ctx context.Context, accountId uuid.UUID, loan model.LoanMetadata) (*model.LoanMetadata, error)
	Delete(ctx context.Context, accountId uuid.UUID) error
}
