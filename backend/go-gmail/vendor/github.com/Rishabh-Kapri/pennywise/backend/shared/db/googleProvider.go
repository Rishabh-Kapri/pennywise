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
		name string,
		picture string,
		email string,
		refreshToken string,
		expiryAt *int64,
	) (*model.UserWithCredentials, error)
	GetUserByGoogleID(ctx context.Context, googleID string) (*model.UserWithCredentials, error)
	GetUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error)
	UpdateHistoryID(ctx context.Context, googleID string, historyID uint64, expiryAt *int64) error
	UpdateHistoryIDByEmail(ctx context.Context, email string, historyID uint64, expiryAt *int64) error
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
	name string,
	picture string,
	email string,
	refreshToken string,
	expiryAt *int64,
) (*model.UserWithCredentials, error) {
	// 1. Create auth_providers linking auth_user to google provider
	_, err := r.Executor(tx).Exec(
		ctx,
		`INSERT INTO auth_providers (auth_user_id, provider_type, provider_id, verified_at)
		 VALUES ($1, 'google', $2, $3)`,
		authUserID, googleID, time.Now(),
	)
	if err != nil {
		return nil, err
	}

	// 2. Create google_provider_users with provider-specific data
	var gpu model.GoogleProviderUser
	err = r.Executor(tx).QueryRow(
		ctx,
		`INSERT INTO google_provider_users (id, name, picture, email, refresh_token, expiry_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, picture, email, gmail_history_id, refresh_token, created_at, updated_at, last_gmail_sync`,
		googleID, name, picture, email, refreshToken, expiryAt,
	).Scan(&gpu.ID, &gpu.Name, &gpu.Picture, &gpu.Email, &gpu.GmailHistoryID,
		&gpu.RefreshToken, &gpu.CreatedAt, &gpu.UpdatedAt, &gpu.LastGmailSync,
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
	var u model.UserWithCredentials
	u.AuthUser = &model.AuthUser{}
	u.GoogleProvider = &model.GoogleProviderUser{}

	err := r.Executor(nil).QueryRow(
		ctx, `
		  SELECT
		    au.id,
		    au.token_version,
		    au.refresh_token_hash,
		    au.created_at,
		    au.updated_at,
		    gpu.id,
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
		  JOIN google_provider_users gpu on ap.provider_id = gpu.id
		  WHERE ap.provider_type = 'google' AND ap.provider_id = $1 AND gpu.deleted = FALSE
		`, googleID,
	).Scan(&u.AuthUser.ID, &u.AuthUser.TokenVersion, &u.AuthUser.RefreshTokenHash,
		&u.AuthUser.CreatedAt, &u.AuthUser.UpdatedAt,
		&u.GoogleProvider.ID, &u.GoogleProvider.Name, &u.GoogleProvider.Picture,
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
	historyID uint64,
	expiryAt *int64,
) error {
	logger.Logger(ctx).Info("updating historyID", "googleID", googleID, "historyID", historyID)
	_, err := r.Executor(nil).Exec(
		ctx, `
		UPDATE google_provider_users 
		SET 
		  gmail_history_id = $1, 
		  last_gmail_sync = NOW(),
			expiry_at = $3
		WHERE 
		  id = $2 AND 
		  deleted = false`,
		historyID, googleID, expiryAt,
	)
	return err
}

func (r *googleProviderRepo) GetUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error) {
	var info model.GoogleUserInfo
	err := r.Executor(nil).QueryRow(
		ctx, `
		  SELECT
		    gpu.id,
		    gpu.email,
		    gpu.gmail_history_id,
		    gpu.refresh_token,
		    gpu.last_gmail_sync,
		    au.id,
		    b.id
		  FROM google_provider_users gpu
		  JOIN auth_providers ap ON ap.provider_id = gpu.id AND ap.provider_type = 'google'
		  JOIN auth_users au ON au.id = ap.auth_user_id
		  JOIN budgets b ON b.user_id = au.id AND b.deleted = FALSE
		  WHERE gpu.email = $1 AND gpu.deleted = FALSE
		  ORDER BY b.is_selected DESC NULLS LAST
		  LIMIT 1
		`, email,
	).Scan(&info.GoogleID, &info.Email, &info.GmailHistoryID, &info.RefreshToken, &info.LastGmailSync, &info.UserID, &info.BudgetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &info, nil
}

func (r *googleProviderRepo) UpdateHistoryIDByEmail(
	ctx context.Context,
	email string,
	historyID uint64,
	expiryAt *int64,
) error {
	query := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Update("google_provider_users").
		Set("gmail_history_id", historyID).
		Set("last_gmail_sync", time.Now()).
		Set("updated_at", time.Now())

	if expiryAt != nil {
		query = query.Set("expiry_at", expiryAt)
	}
	query = query.Where(sq.Eq{"email": email, "deleted": false})

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}
	_, err = r.Executor(nil).Exec(ctx, sql, args...)
	return err
}
