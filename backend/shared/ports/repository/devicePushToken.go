package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type DevicePushTokenRepository interface {
	// Upsert registers a token for a user, refreshing last_seen_at (and owner)
	// when the token is already known.
	Upsert(ctx context.Context, authUserId uuid.UUID, token string, platform string) (*model.DevicePushToken, error)
	// GetByBudgetId returns all tokens belonging to the owner of a budget.
	GetByBudgetId(ctx context.Context, budgetId uuid.UUID) ([]model.DevicePushToken, error)
	DeleteByToken(ctx context.Context, authUserId uuid.UUID, token string) error
}
