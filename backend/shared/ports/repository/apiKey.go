package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type APIKeyRepository interface {
	Create(ctx context.Context, apiKey *model.APIKey) error
	GetAll(ctx context.Context, userID uuid.UUID) ([]model.APIKey, error)
	GetByKeyID(ctx context.Context, keyID string) (*model.APIKey, error)
	GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error)
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}
