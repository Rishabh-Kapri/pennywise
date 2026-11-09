package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type googleProviderRepo struct {
	BaseRepository
}

func NewGoogleProviderRepository(pool *pgxpool.Pool) repository.GoogleProviderRepository {
	return &googleProviderRepo{BaseRepository: NewBaseRepository(pool)}
}

// Create inserts an auth_providers row and a google_provider_users row within the given transaction.
func (r *googleProviderRepo) Create(
	ctx context.Context,
	authUserID uuid.UUID,
	googleID string,
	oauthClientType model.GoogleOAuthClientType,
	name string,
	picture string,
	email string,
) (*model.UserWithCredentials, error) {
	oauthClientType = model.NormalizeGoogleOAuthClientType(oauthClientType)
	// 1. Create auth_providers linking auth_user to google provider. Lookups
	// skip soft-deleted rows but the unique index still counts them, so a
	// plain INSERT would fail forever once a row for this (google id, client
	// type) has existed; resurrect and reattach it instead.
	_, err := r.Executor(ctx).Exec(
		ctx,
		`INSERT INTO auth_providers (auth_user_id, provider_type, provider_id, oauth_client_type, verified_at)
		 VALUES ($1, 'google', $2, $3, $4)
		 ON CONFLICT (provider_id, oauth_client_type) DO UPDATE SET
		   auth_user_id = EXCLUDED.auth_user_id,
		   verified_at = EXCLUDED.verified_at,
		   deleted = false,
		   updated_at = now()`,
		authUserID, googleID, oauthClientType, time.Now(),
	)
	if err != nil {
		return nil, err
	}

	// 2. Create google_provider_users with provider-specific data.
	var gpu model.GoogleProviderUser
	err = r.Executor(ctx).QueryRow(
		ctx,
		`INSERT INTO google_provider_users (id, oauth_client_type, name, picture, email)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id, oauth_client_type) DO UPDATE SET
		   name = EXCLUDED.name,
		   picture = EXCLUDED.picture,
		   email = EXCLUDED.email,
		   deleted = false,
		   updated_at = now()
		 RETURNING id, oauth_client_type, name, picture, email, created_at, updated_at`,
		googleID, oauthClientType, name, picture, email,
	).Scan(&gpu.ID, &gpu.OAuthClientType, &gpu.Name, &gpu.Picture, &gpu.Email, &gpu.CreatedAt, &gpu.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &model.UserWithCredentials{
		AuthUser:       &model.AuthUser{ID: authUserID},
		GoogleProvider: &gpu,
	}, nil
}

func (r *googleProviderRepo) GetUserByGoogleID(
	ctx context.Context,
	googleID string,
) (*model.UserWithCredentials, error) {
	return r.getUserByGoogleID(ctx, googleID, "", false)
}

func (r *googleProviderRepo) GetUserByGoogleIDAndClientType(
	ctx context.Context,
	googleID string,
	oauthClientType model.GoogleOAuthClientType,
) (*model.UserWithCredentials, error) {
	return r.getUserByGoogleID(ctx, googleID, model.NormalizeGoogleOAuthClientType(oauthClientType), true)
}

func (r *googleProviderRepo) getUserByGoogleID(
	ctx context.Context,
	googleID string,
	oauthClientType model.GoogleOAuthClientType,
	filterByClientType bool,
) (*model.UserWithCredentials, error) {
	var u model.UserWithCredentials
	u.AuthUser = &model.AuthUser{}
	u.GoogleProvider = &model.GoogleProviderUser{}

	query := `
		  SELECT
		    au.id,
		    au.token_version,
		    au.refresh_token_hash,
		    au.created_at,
		    au.updated_at,
		    gpu.id,
		    gpu.oauth_client_type,
		    gpu.name,
		    gpu.picture,
		    gpu.email,
		    gpu.created_at,
		    gpu.updated_at
		  FROM auth_users au
		  JOIN auth_providers ap ON au.id = ap.auth_user_id
		  JOIN google_provider_users gpu on ap.provider_id = gpu.id AND ap.oauth_client_type = gpu.oauth_client_type
		  WHERE ap.provider_type = 'google' AND ap.provider_id = $1 AND gpu.deleted = FALSE AND ap.deleted = FALSE`
	args := []any{googleID}
	if filterByClientType {
		query += ` AND ap.oauth_client_type = $2`
		args = append(args, oauthClientType)
	}
	query += ` ORDER BY ap.verified_at ASC NULLS LAST LIMIT 1`

	err := r.Executor(ctx).QueryRow(ctx, query, args...).Scan(&u.AuthUser.ID, &u.AuthUser.TokenVersion, &u.AuthUser.RefreshTokenHash,
		&u.AuthUser.CreatedAt, &u.AuthUser.UpdatedAt,
		&u.GoogleProvider.ID, &u.GoogleProvider.OAuthClientType, &u.GoogleProvider.Name, &u.GoogleProvider.Picture,
		&u.GoogleProvider.Email, &u.GoogleProvider.CreatedAt, &u.GoogleProvider.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *googleProviderRepo) GetUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error) {
	var info model.GoogleUserInfo
	err := r.Executor(ctx).QueryRow(
		ctx, `
		  SELECT
		    gpu.id,
		    gpu.oauth_client_type,
		    gpu.email,
		    au.id,
		    b.id
		  FROM google_provider_users gpu
		  JOIN auth_providers ap ON ap.provider_id = gpu.id AND ap.oauth_client_type = gpu.oauth_client_type AND ap.provider_type = 'google'
		  JOIN auth_users au ON au.id = ap.auth_user_id
		  JOIN budgets b ON b.user_id = au.id AND b.deleted = FALSE
		  WHERE gpu.email = $1 AND gpu.deleted = FALSE
		  ORDER BY b.is_selected DESC NULLS LAST,
		           gpu.updated_at DESC
		  LIMIT 1
		`, email,
	).Scan(&info.GoogleID, &info.OAuthClientType, &info.Email, &info.UserID, &info.BudgetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrUserNotFound
		}
		return nil, err
	}
	return &info, nil
}

func (r *googleProviderRepo) UpdateUserByGoogleIDAndClientType(
	ctx context.Context,
	googleID string,
	oauthClientType model.GoogleOAuthClientType,
	data *model.GoogleProviderUser,
) error {
	oauthClientType = model.NormalizeGoogleOAuthClientType(oauthClientType)
	logger.Logger(ctx).Info("updating user", "googleID", googleID, "oauthClientType", oauthClientType)
	query := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Update("google_provider_users")
	isUpdate := false
	if data.Name != "" {
		isUpdate = true
		query = query.Set("name", data.Name)
	}
	if data.Picture != "" {
		isUpdate = true
		query = query.Set("picture", data.Picture)
	}
	if isUpdate {
		query = query.Set("updated_at", time.Now())
		query = query.Where(sq.Eq{"id": googleID, "oauth_client_type": oauthClientType, "deleted": false})
		sql, args, err := query.ToSql()
		if err != nil {
			return err
		}
		_, err = r.Executor(ctx).Exec(ctx, sql, args...)
		return err
	}
	return nil
}
