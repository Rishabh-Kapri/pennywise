package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type CategoryGroupRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.CategoryGroup, error)
	Create(ctx context.Context, categoryGroup model.CategoryGroup) (*model.CategoryGroup, error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, categoryGroup model.CategoryGroup) error
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}
