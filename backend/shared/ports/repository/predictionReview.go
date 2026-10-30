package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

// PredictionReviewRepository serves the cipher-prediction review queue. It is
// separate from CipherPredictionRepository so the ingestion path's interface
// stays focused on writing predictions.
type PredictionReviewRepository interface {
	GetQueue(ctx context.Context, budgetID uuid.UUID, includeReviewed bool, limit int) ([]model.PredictionReviewItem, error)
	GetByID(ctx context.Context, budgetID uuid.UUID, id uuid.UUID) (*model.PredictionReviewItem, error)
	// MarkReviewed stamps reviewed_at. When actual ids are supplied (the
	// accept path) they are recorded as the confirmed labels.
	MarkReviewed(ctx context.Context, budgetID uuid.UUID, id uuid.UUID, actualPayeeID, actualCategoryID *uuid.UUID) error
}
