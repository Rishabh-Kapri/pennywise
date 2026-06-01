package db

import (
	"context"
	"errors"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GoogleProviderRepository interface {
	BaseRepositoryInterface
	GetAll(ctx context.Context, tx pgx.Tx) ([]model.GoogleProviderUser, error)
	Create(
		ctx context.Context,
		tx pgx.Tx,
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

type googleProviderRepo struct {
	BaseRepository
}

func NewGoogleProviderRepository(pool *pgxpool.Pool) GoogleProviderRepository {
	return &googleProviderRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *googleProviderRepo) GetAll(ctx context.Context, tx pgx.Tx) ([]model.GoogleProviderUser, error) {
	rows, err := r.Executor(tx).Query(ctx, `
		SELECT 
		  id,
		  oauth_client_type,
		  name,
		  picture, 
		  email, 
		  gmail_history_id, 
		  refresh_token, 
		  created_at, 
		  updated_at, 
		  last_gmail_sync, 
		  expiry_at
		FROM google_provider_users
		WHERE deleted = FALSE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.GoogleProviderUser
	for rows.Next() {
		var user model.GoogleProviderUser
		if err := rows.Scan(
			&user.ID,
			&user.OAuthClientType,
			&user.Name,
			&user.Picture,
			&user.Email,
			&user.GmailHistoryID,
			&user.RefreshToken,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.LastGmailSync,
			&user.ExpiryAt,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

// Create inserts an auth_providers row and a google_provider_users row within the given transaction.
func (r *googleProviderRepo) Create(
	ctx context.Context,
	tx pgx.Tx,
	authUserID uuid.UUID,
	googleID string,
	oauthClientType model.GoogleOAuthClientType,
	name string,
	picture string,
	email string,
	refreshToken string,
	expiryAt *int64,
) (*model.UserWithCredentials, error) {
	oauthClientType = model.NormalizeGoogleOAuthClientType(oauthClientType)
	// 1. Create auth_providers linking auth_user to google provider
	_, err := r.Executor(tx).Exec(
		ctx,
		`INSERT INTO auth_providers (auth_user_id, provider_type, provider_id, oauth_client_type, verified_at)
		 VALUES ($1, 'google', $2, $3, $4)`,
		authUserID, googleID, oauthClientType, time.Now(),
	)
	if err != nil {
		return nil, err
	}

	// 2. Create google_provider_users with provider-specific data
	var gpu model.GoogleProviderUser
	err = r.Executor(tx).QueryRow(
		ctx,
		`INSERT INTO google_provider_users (id, oauth_client_type, name, picture, email, refresh_token, expiry_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, oauth_client_type, name, picture, email, gmail_history_id, refresh_token, created_at, updated_at, last_gmail_sync, expiry_at`,
		googleID, oauthClientType, name, picture, email, refreshToken, expiryAt,
	).Scan(&gpu.ID, &gpu.OAuthClientType, &gpu.Name, &gpu.Picture, &gpu.Email, &gpu.GmailHistoryID,
		&gpu.RefreshToken, &gpu.CreatedAt, &gpu.UpdatedAt, &gpu.LastGmailSync, &gpu.ExpiryAt,
	)
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
		    gpu.gmail_history_id,
		    gpu.refresh_token,
		    gpu.created_at,
		    gpu.updated_at,
		    gpu.last_gmail_sync,
			  gpu.expiry_at
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

	err := r.Executor(nil).QueryRow(ctx, query, args...).Scan(&u.AuthUser.ID, &u.AuthUser.TokenVersion, &u.AuthUser.RefreshTokenHash,
		&u.AuthUser.CreatedAt, &u.AuthUser.UpdatedAt,
		&u.GoogleProvider.ID, &u.GoogleProvider.OAuthClientType, &u.GoogleProvider.Name, &u.GoogleProvider.Picture,
		&u.GoogleProvider.Email, &u.GoogleProvider.GmailHistoryID,
		&u.GoogleProvider.RefreshToken, &u.GoogleProvider.CreatedAt,
		&u.GoogleProvider.UpdatedAt, &u.GoogleProvider.LastGmailSync, &u.GoogleProvider.ExpiryAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *googleProviderRepo) UpdateHistoryID(
	ctx context.Context,
	googleID string,
	oauthClientType model.GoogleOAuthClientType,
	historyID uint64,
	expiryAt *int64,
) error {
	oauthClientType = model.NormalizeGoogleOAuthClientType(oauthClientType)
	logger.Logger(ctx).Info("updating historyID", "googleID", googleID, "oauthClientType", oauthClientType, "historyID", historyID)
	query := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Update("google_provider_users").
		Set("gmail_history_id", historyID).
		Set("last_gmail_sync", time.Now()).
		Set("updated_at", time.Now())

	if expiryAt != nil {
		query = query.Set("expiry_at", expiryAt)
	}
	query = query.Where(sq.Eq{"id": googleID, "oauth_client_type": oauthClientType, "deleted": false})

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = r.Executor(nil).Exec(ctx, sql, args...)
	return err
}

func (r *googleProviderRepo) GetUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error) {
	var info model.GoogleUserInfo
	err := r.Executor(nil).QueryRow(
		ctx, `
		  SELECT
		    gpu.id,
		    gpu.oauth_client_type,
		    gpu.email,
		    gpu.gmail_history_id,
		    gpu.refresh_token,
		    gpu.last_gmail_sync,
		    au.id,
		    b.id
		  FROM google_provider_users gpu
		  JOIN auth_providers ap ON ap.provider_id = gpu.id AND ap.oauth_client_type = gpu.oauth_client_type AND ap.provider_type = 'google'
		  JOIN auth_users au ON au.id = ap.auth_user_id
		  JOIN budgets b ON b.user_id = au.id AND b.deleted = FALSE
		  WHERE gpu.email = $1 AND gpu.deleted = FALSE
		  ORDER BY (gpu.gmail_history_id IS NOT NULL) DESC,
		           gpu.last_gmail_sync DESC NULLS LAST,
		           b.is_selected DESC NULLS LAST,
		           gpu.updated_at DESC
		  LIMIT 1
		`, email,
	).Scan(&info.GoogleID, &info.OAuthClientType, &info.Email, &info.GmailHistoryID, &info.RefreshToken, &info.LastGmailSync, &info.UserID, &info.BudgetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
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
	logger.Logger(ctx).Info("updating user", "googleID", googleID, "oauthClientType", oauthClientType, "data", data)
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
	if data.RefreshToken != "" {
		isUpdate = true
		query = query.Set("refresh_token", data.RefreshToken)
	}
	if isUpdate {
		query = query.Set("updated_at", time.Now())
		query = query.Where(sq.Eq{"id": googleID, "oauth_client_type": oauthClientType, "deleted": false})
		sql, args, err := query.ToSql()
		if err != nil {
			return err
		}
		_, err = r.Executor(nil).Exec(ctx, sql, args...)
		return err
	}
	return nil
}

func (r *googleProviderRepo) UpdateHistoryIDByEmail(
	ctx context.Context,
	email string,
	oauthClientType model.GoogleOAuthClientType,
	historyID uint64,
	expiryAt *int64,
) error {
	oauthClientType = model.NormalizeGoogleOAuthClientType(oauthClientType)
	query := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Update("google_provider_users").
		Set("gmail_history_id", historyID).
		Set("last_gmail_sync", time.Now()).
		Set("updated_at", time.Now())

	if expiryAt != nil {
		query = query.Set("expiry_at", expiryAt)
	}
	query = query.Where(sq.Eq{"email": email, "oauth_client_type": oauthClientType, "deleted": false})

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}
	_, err = r.Executor(nil).Exec(ctx, sql, args...)
	return err
}
