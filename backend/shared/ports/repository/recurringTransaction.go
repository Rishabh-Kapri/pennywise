package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type RecurringTransactionRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.RecurringTransaction, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.RecurringTransaction, error)
	// GetDue returns rules ready to fire on or before today, across every
	// budget, so the background scheduler can run a single scan.
	GetDue(ctx context.Context, today string, budgetId *uuid.UUID) ([]model.RecurringTransaction, error)
	Create(ctx context.Context, budgetId uuid.UUID, rule model.RecurringTransaction) (*model.RecurringTransaction, error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, rule model.RecurringTransaction) error
	MarkRun(ctx context.Context, id uuid.UUID, nextDate model.Date, lastRunDate model.Date) error
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}
