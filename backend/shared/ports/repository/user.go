package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type UserRepository interface {
	// Create(ctx context.Context, user model.User) (*model.User, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.User, error)
	Update(ctx context.Context, budgetId uuid.UUID, user model.User) (*model.User, error)
}
