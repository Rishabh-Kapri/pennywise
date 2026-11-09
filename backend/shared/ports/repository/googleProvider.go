package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type GoogleProviderRepository interface {
	Create(
		ctx context.Context,
		authUserID uuid.UUID,
		googleID string,
		oauthClientType model.GoogleOAuthClientType,
		name string,
		picture string,
		email string,
	) (*model.UserWithCredentials, error)
	GetUserByGoogleID(ctx context.Context, googleID string) (*model.UserWithCredentials, error)
	GetUserByGoogleIDAndClientType(ctx context.Context, googleID string, oauthClientType model.GoogleOAuthClientType) (*model.UserWithCredentials, error)
	GetUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error)
	UpdateUserByGoogleIDAndClientType(ctx context.Context, googleID string, oauthClientType model.GoogleOAuthClientType, data *model.GoogleProviderUser) error
}
