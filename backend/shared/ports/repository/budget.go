package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type BudgetRepository interface {
	GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error)
	GetById(ctx context.Context, id uuid.UUID) (*model.Budget, error)
	Create(ctx context.Context, name string, userID uuid.UUID) (*model.Budget, error)
	UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error
	IsOwnedByUser(ctx context.Context, budgetID uuid.UUID, userID uuid.UUID) (bool, error)
	UpdateOnboarding(ctx context.Context, id uuid.UUID, onboarding model.BudgetOnboarding) error
}
