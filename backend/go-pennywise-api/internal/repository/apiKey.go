package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type APIKeyRepository interface {
	db.BaseRepositoryInterface
	Create(ctx context.Context, tx pgx.Tx, apiKey *model.APIKey) error
	GetByKeyID(ctx context.Context, tx pgx.Tx, keyID string) (*model.APIKey, error)
	GetByHash(ctx context.Context, tx pgx.Tx, keyHash string) (*model.APIKey, error)
}

type apiKeyRepo struct {
	db.BaseRepository
}

func NewAPIKeyRepository(pool *pgxpool.Pool) APIKeyRepository {
	return &apiKeyRepo{
		BaseRepository: db.NewBaseRepository(pool),
	}
}

// Create stores a new API key
func (r *apiKeyRepo) Create(ctx context.Context, tx pgx.Tx, key *model.APIKey) error {
	query := `
    INSERT INTO api_keys (
        key_id, hashed_key, name, description, user_id, scopes, allowed_ips, allowed_referers, rate_limit, expires_at
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    RETURNING id, created_at
    `
	err := r.Executor(tx).QueryRow(ctx, query,
		key.KeyID,
		key.HashedKey,
		key.Name,
		key.Description,
		key.UserID,
		key.Scopes,
		key.AllowedIPs,
		key.AllowedReferers,
		key.RateLimit,
		key.ExpiresAt,
	).Scan(&key.ID, &key.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *apiKeyRepo) GetByKeyID(ctx context.Context, tx pgx.Tx, keyID string) (*model.APIKey, error) {
	var key model.APIKey
	err := r.Executor(tx).QueryRow(
		ctx,
		`SELECT 
		  id,
		  key_id,
		  hashed_key,
		  name,
			description,
		  user_id,
		  scopes,
		  allowed_ips,
		  allowed_referers,
		  rate_limit,
		  expires_at,
		  rotation_due_at,
		  created_at,
		  expires_at,
		  last_used_at,
		  revoked_at,
		  rotation_enabled,
		  rotated_from_id,
		  rotation_due_at,
		  is_active
		FROM api_keys
		WHERE key_id = $1`,
		keyID,
	).Scan(
		&key.ID,
		&key.KeyID,
		&key.HashedKey,
		&key.Name,
		&key.Description,
		&key.UserID,
		&key.Scopes,
		&key.AllowedIPs,
		&key.AllowedReferers,
		&key.RateLimit,
		&key.ExpiresAt,
		&key.RotationDueAt,
		&key.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *apiKeyRepo) GetByHash(ctx context.Context, tx pgx.Tx, keyHash string) (*model.APIKey, error) {
	var key model.APIKey
	err := r.Executor(tx).QueryRow(
		ctx,
		`SELECT 
		  id,
		  key_id,
		  hashed_key,
		  name,
			description,
		  user_id,
		  scopes,
		  allowed_ips,
		  allowed_referers,
		  rate_limit,
		  rotation_due_at,
		  created_at,
		  expires_at,
		  last_used_at,
		  revoked_at,
		  rotation_enabled,
		  rotated_from_id,
		  rotation_due_at,
		  is_active
		FROM api_keys
		WHERE hashed_key = $1`,
		keyHash,
	).Scan(
		&key.ID,
		&key.KeyID,
		&key.HashedKey,
		&key.Name,
		&key.Description,
		&key.UserID,
		&key.Scopes,
		&key.AllowedIPs,
		&key.AllowedReferers,
		&key.RateLimit,
		&key.RotationDueAt,
		&key.CreatedAt,
		&key.ExpiresAt,
		&key.LastUsedAt,
		&key.RevokedAt,
		&key.RotationEnabled,
		&key.RotatedFromID,
		&key.RotationDueAt,
		&key.IsActive,
	)
	logger.Logger(ctx).Debug("GetByHash", "keyHash", keyHash, "key", key, "err", err)
	if err != nil {
		return nil, err
	}
	return &key, nil
}
