package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type CipherPredictionRepository interface {
	Create(ctx context.Context, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error)
	GetAll(ctx context.Context, budgetID uuid.UUID) ([]model.CipherPredictionRecord, error)
	GetByTransactionID(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID) (*model.CipherPredictionRecord, error)
	MarkUserCorrected(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID, actualPayeeID *uuid.UUID, actualCategoryID *uuid.UUID) error
	MarkLearned(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID) error
	// ClaimLearning stamps learned_at if nothing has yet, and reports whether
	// this call did. Learning from a prediction once relies on it.
	ClaimLearning(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID) (bool, error)
	// MarkReviewed stamps reviewed_at unless it's set. A transaction without a
	// prediction is no error.
	MarkReviewed(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID) error
	// DeleteByTransactionID soft-deletes the transaction's prediction, if any.
	DeleteByTransactionID(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID) error
	MarkLearnFailed(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID, reason string) error
	ListPendingLearning(ctx context.Context, budgetID uuid.UUID, limit int, retryFailed bool) ([]model.LearningCandidate, error)
	LearningStats(ctx context.Context, budgetID uuid.UUID) (model.LearningStats, error)
}
