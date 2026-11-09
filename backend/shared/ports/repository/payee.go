package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type PayeesRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Payee, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Payee, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Payee, error)
	Create(ctx context.Context, payee model.Payee) (*model.Payee, error)
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payee model.Payee) error
	GetStats(ctx context.Context, budgetId uuid.UUID) ([]model.PayeeStats, error)
	Merge(ctx context.Context, budgetId, targetId uuid.UUID, sourceIds []uuid.UUID) (*model.PayeeMergeResult, error)
}
