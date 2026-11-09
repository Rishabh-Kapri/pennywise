package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type PredictionRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Prediction, error)
	GetByTxnId(ctx context.Context, budgetId uuid.UUID, txnId uuid.UUID) (*model.Prediction, error)
	Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, prediction model.Prediction) error
	// DeleteById deletes a prediction by budget and txn id.
	DeleteByTxnId(ctx context.Context, budgetId uuid.UUID, txnId uuid.UUID) error
}
