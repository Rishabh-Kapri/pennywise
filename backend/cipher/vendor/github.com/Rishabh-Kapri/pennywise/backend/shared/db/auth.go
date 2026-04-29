package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

type AuthRepository interface {
	BaseRepositoryInterface
	CreateUser(ctx context.Context, tx pgx.Tx) (*model.AuthUser, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.AuthUser, error)
	GetUserWithProviders(ctx context.Context, id uuid.UUID) (*model.CurrentAuthUserResponse, error)
	UpdateTokenVersion(ctx context.Context, id uuid.UUID) error
	GetTokenVersion(ctx context.Context, id uuid.UUID) (int, error)
	SaveRefreshTokenHash(ctx context.Context, userID uuid.UUID, tokenHash string) error
	GetRefreshTokenHash(ctx context.Context, userID uuid.UUID) (string, error)
	ClearRefreshTokenHash(ctx context.Context, userID uuid.UUID) error
}

type authRepo struct {
	BaseRepository
}

func NewAuthRepository(pool *pgxpool.Pool) AuthRepository {
	return &authRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *authRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.AuthUser, error) {
	var user model.AuthUser
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT id, token_version, refresh_token_hash, created_at, updated_at FROM auth_users WHERE id = $1 AND deleted = false`,
		id,
	).
		Scan(&user.ID, &user.TokenVersion, &user.RefreshTokenHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *authRepo) GetUserWithProviders(ctx context.Context, id uuid.UUID) (*model.CurrentAuthUserResponse, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT
			au.id,
			au.created_at,
			au.updated_at,
			ap.provider_type,
			ap.provider_id,
			ap.verified_at
		FROM auth_users au
		LEFT JOIN auth_providers ap ON ap.auth_user_id = au.id
		WHERE au.id = $1 AND au.deleted = false
		ORDER BY ap.verified_at ASC NULLS LAST`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var user *model.CurrentAuthUserResponse
	for rows.Next() {
		var userID uuid.UUID
		var userCreatedAt time.Time
		var userUpdatedAt time.Time
		var providerType sql.NullString
		var providerID sql.NullString
		var verifiedAt sql.NullTime

		if err := rows.Scan(
			&userID,
			&userCreatedAt,
			&userUpdatedAt,
			&providerType,
			&providerID,
			&verifiedAt,
		); err != nil {
			return nil, err
		}

		if user == nil {
			user = &model.CurrentAuthUserResponse{
				ID:        userID,
				CreatedAt: userCreatedAt,
				UpdatedAt: userUpdatedAt,
				Providers: []model.AuthProviderUserResponse{},
			}
		}

		if !providerType.Valid || !providerID.Valid {
			continue
		}

		provider := model.AuthProviderUserResponse{
			ProviderType: model.AuthProviderType(providerType.String),
			ProviderID:   providerID.String,
		}
		if verifiedAt.Valid {
			provider.VerifiedAt = verifiedAt.Time
		}
		user.Providers = append(user.Providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return user, nil
}

func (r *authRepo) CreateUser(ctx context.Context, tx pgx.Tx) (*model.AuthUser, error) {
	var user model.AuthUser
	err := r.Executor(tx).QueryRow(
		ctx,
		`INSERT INTO auth_users (token_version) VALUES (1)
		 RETURNING id, token_version, refresh_token_hash, created_at, updated_at`,
	).Scan(&user.ID, &user.TokenVersion, &user.RefreshTokenHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepo) UpdateTokenVersion(ctx context.Context, id uuid.UUID) error {
	_, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE auth_users SET token_version = token_version + 1, updated_at = NOW() WHERE id = $1 AND deleted = false`,
		id,
	)
	return err
}

func (r *authRepo) GetTokenVersion(ctx context.Context, id uuid.UUID) (int, error) {
	var tokenVersion int
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT token_version FROM auth_users WHERE id = $1 AND deleted = false`,
		id,
	).Scan(&tokenVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrUserNotFound
		}
		return 0, err
	}
	return tokenVersion, nil
}

func (r *authRepo) SaveRefreshTokenHash(ctx context.Context, userID uuid.UUID, tokenHash string) error {
	_, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE auth_users SET refresh_token_hash = $1, updated_at = NOW() WHERE id = $2 AND deleted = false`,
		tokenHash, userID,
	)
	return err
}

func (r *authRepo) GetRefreshTokenHash(ctx context.Context, userID uuid.UUID) (string, error) {
	var hash sql.NullString
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT refresh_token_hash FROM auth_users WHERE id = $1 AND deleted = false`,
		userID,
	).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", err
	}
	return hash.String, nil
}

func (r *authRepo) ClearRefreshTokenHash(ctx context.Context, userID uuid.UUID) error {
	_, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE auth_users SET refresh_token_hash = NULL, updated_at = NOW() WHERE id = $1 AND deleted = false`,
		userID,
	)
	return err
}
