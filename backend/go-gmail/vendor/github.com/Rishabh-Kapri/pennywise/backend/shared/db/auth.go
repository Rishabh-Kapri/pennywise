package db

import (
	"context"
	"database/sql"
	"errors"

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
