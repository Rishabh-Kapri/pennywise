package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type TagRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Tag, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Tag, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Tag, error)
	Create(ctx context.Context, tag model.Tag) (*model.Tag, error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, tag model.Tag) error
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}
