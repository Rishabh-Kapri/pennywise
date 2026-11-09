package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type MonthlyBudgetRepository interface {
	GetByCatIdAndMonth(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, month string) (*model.MonthlyBudget, error)
	Create(ctx context.Context, budgetId uuid.UUID, monthlyBudget model.MonthlyBudget) error
	UpdateBudgetedByCatIdAndMonth(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, month string, newBudgeted float64) error
	// updates the carryover for current and further months
	// amount should be the value the carryvover needs to be updated with
	UpdateCarryoverByCatIdAndMonth(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, month string, amount float64) error
}
