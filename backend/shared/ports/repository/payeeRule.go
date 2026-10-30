package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type PayeeRuleRepository interface {
	CreatePayeeRule(ctx context.Context, payeeMatch model.PayeeRule) error
	FindByMatchString(ctx context.Context, budgetId uuid.UUID, matchString string) (*model.PayeeRule, error)
	FindByPayeeID(ctx context.Context, budgetId uuid.UUID, payeeId uuid.UUID) ([]model.PayeeRuleDetails, error)
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.PayeeRuleDetails, error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payeeRule model.PayeeRule) error
	DeleteByID(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}
