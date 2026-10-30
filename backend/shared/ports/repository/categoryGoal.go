package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

// CategoryGoalRepository stores category targets (category_goals), at most one
// per category.
type CategoryGoalRepository interface {
	// Upsert creates or replaces the category's target. It returns
	// ErrNotFound when the category is not in the budget, is deleted, or is
	// the inflow (system) category, which cannot carry a target.
	Upsert(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, goal model.CategoryGoal) error
	// Delete removes the category's target; ErrNotFound when it has none.
	Delete(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID) error
}
