package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")
type AuthRepository interface {
	BaseRepository
	FindByGoogleID(ctx context.Context, googleID string) (*model.AuthUser, error)
	FindByEmail(ctx context.Context, email string) (*model.AuthUser, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.AuthUser, error)
	Create(ctx context.Context, user model.AuthUser) (*model.AuthUser, error)
	UpdateTokenVersion(ctx context.Context, id uuid.UUID) error
	GetTokenVersion(ctx context.Context, id uuid.UUID) (int, error)
	SaveRefreshTokenHash(ctx context.Context, userID uuid.UUID, tokenHash string) error
	GetRefreshTokenHash(ctx context.Context, userID uuid.UUID) (string, error)
	ClearRefreshTokenHash(ctx context.Context, userID uuid.UUID) error
}

type authRepo struct {
	baseRepository
}

func NewAuthRepository(db *pgxpool.Pool) AuthRepository {
	return &authRepo{baseRepository: NewBaseRepository(db)}
}

func (r *authRepo) FindByGoogleID(ctx context.Context, googleID string) (*model.AuthUser, error) {
	var user model.AuthUser
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT id, google_id, email, name, picture, token_version, created_at, updated_at 
		 FROM auth_users WHERE google_id = $1 AND deleted = false`,
		googleID,
	).Scan(&user.ID, &user.GoogleID, &user.Email, &user.Name, &user.Picture, &user.TokenVersion, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *authRepo) FindByEmail(ctx context.Context, email string) (*model.AuthUser, error) {
	var user model.AuthUser
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT id, google_id, email, name, picture, token_version, created_at, updated_at 
		 FROM auth_users WHERE email = $1 AND deleted = false`,
		email,
	).Scan(&user.ID, &user.GoogleID, &user.Email, &user.Name, &user.Picture, &user.TokenVersion, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *authRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.AuthUser, error) {
	var user model.AuthUser
	var picture sql.NullString
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT id, google_id, email, name, picture, token_version, created_at, updated_at 
		 FROM auth_users WHERE id = $1 AND deleted = false`,
		id,
	).Scan(&user.ID, &user.GoogleID, &user.Email, &user.Name, &picture, &user.TokenVersion, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	user.Picture = picture.String
	return &user, nil
}

func (r *authRepo) Create(ctx context.Context, user model.AuthUser) (*model.AuthUser, error) {
	var createdUser model.AuthUser
	var picture sql.NullString
	err := r.Executor(nil).QueryRow(
		ctx,
		`INSERT INTO auth_users (google_id, email, name, picture, token_version) 
		 VALUES ($1, $2, $3, $4, 1) 
		 RETURNING id, google_id, email, name, picture, token_version, created_at, updated_at`,
		user.GoogleID, user.Email, user.Name, user.Picture,
	).Scan(&createdUser.ID, &createdUser.GoogleID, &createdUser.Email, &createdUser.Name, &picture, &createdUser.TokenVersion, &createdUser.CreatedAt, &createdUser.UpdatedAt)

	if err != nil {
		return nil, err
	}
	createdUser.Picture = picture.String
	return &createdUser, nil
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
