package repository

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

var ErrUserNotFound = errors.New("user not found")

type AuthRepository interface {
	CreateUser(ctx context.Context) (*model.AuthUser, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.AuthUser, error)
	GetUserWithProviders(ctx context.Context, id uuid.UUID) (*model.CurrentAuthUserResponse, error)
	UpdateTokenVersion(ctx context.Context, id uuid.UUID) error
	GetTokenVersion(ctx context.Context, id uuid.UUID) (int, error)
	SaveRefreshTokenHash(ctx context.Context, userID uuid.UUID, tokenHash string) error
	GetRefreshTokenHash(ctx context.Context, userID uuid.UUID) (string, error)
	ClearRefreshTokenHash(ctx context.Context, userID uuid.UUID) error
}
