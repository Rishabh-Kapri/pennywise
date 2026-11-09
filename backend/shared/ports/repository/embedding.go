package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type EmbeddingRepository interface {
	Get(ctx context.Context, docType string, embedding []float64, limit int64) ([]model.Embedding, error)
	Create(ctx context.Context, data model.Embedding, embedding []float64) error
}
