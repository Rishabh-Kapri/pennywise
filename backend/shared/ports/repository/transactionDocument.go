package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type TransactionDocumentRepository interface {
	GetByTransactionId(ctx context.Context, budgetId uuid.UUID, transactionId uuid.UUID) ([]model.TransactionDocument, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.TransactionDocument, error)
	// Search lists every document in the budget with its transaction context.
	Search(ctx context.Context, budgetId uuid.UUID, filter model.DocumentFilter) (model.DocumentLibraryResponse, error)
	Create(ctx context.Context, doc model.TransactionDocument) (*model.TransactionDocument, error)
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}
