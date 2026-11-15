package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type BudgetRepository interface {
	// GetAll lists the budgets userID is a member of, each with their role,
	// their selection and the number of members.
	GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error)
	GetById(ctx context.Context, id uuid.UUID) (*model.Budget, error)
	// Create makes the budget and userID its owner (a budget_members row).
	// It's selected when the user has no other budget selected.
	Create(ctx context.Context, name string, userID uuid.UUID) (*model.Budget, error)
	// UpdateById writes the name and metadata. Selection is per member:
	// BudgetMemberRepository.SetSelected.
	UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error
	Rename(ctx context.Context, id uuid.UUID, name string) error
	UpdateOnboarding(ctx context.Context, id uuid.UUID, onboarding model.BudgetOnboarding) error
}
