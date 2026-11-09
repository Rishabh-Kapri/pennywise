package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type TransactionEmbeddingRepository interface {
	SearchSimilar(ctx context.Context, budgetID uuid.UUID, amount float64, embedding []float64, limit int) ([]model.TransactionEmbedding, error)
	Upsert(ctx context.Context, data model.TransactionEmbedding, embedding []float64) error
}
