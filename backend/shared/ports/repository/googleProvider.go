package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type GoogleProviderRepository interface {
	GetAll(ctx context.Context) ([]model.GoogleProviderUser, error)
	Create(
		ctx context.Context,
		authUserID uuid.UUID,
		googleID string,
		oauthClientType model.GoogleOAuthClientType,
		name string,
		picture string,
		email string,
		refreshToken string,
		expiryAt *int64,
	) (*model.UserWithCredentials, error)
	GetUserByGoogleID(ctx context.Context, googleID string) (*model.UserWithCredentials, error)
	GetUserByGoogleIDAndClientType(ctx context.Context, googleID string, oauthClientType model.GoogleOAuthClientType) (*model.UserWithCredentials, error)
	GetUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error)
	UpdateUserByGoogleIDAndClientType(ctx context.Context, googleID string, oauthClientType model.GoogleOAuthClientType, data *model.GoogleProviderUser) error
	UpdateHistoryID(ctx context.Context, googleID string, oauthClientType model.GoogleOAuthClientType, historyID uint64, expiryAt *int64) error
	UpdateHistoryIDByEmail(ctx context.Context, email string, oauthClientType model.GoogleOAuthClientType, historyID uint64, expiryAt *int64) error
}
