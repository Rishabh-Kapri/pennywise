package postgres

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type apiKeyRepo struct {
	BaseRepository
}

func NewAPIKeyRepository(pool *pgxpool.Pool) repository.APIKeyRepository {
	return &apiKeyRepo{
		BaseRepository: NewBaseRepository(pool),
	}
}

// Create stores a new API key
func (r *apiKeyRepo) Create(ctx context.Context, key *model.APIKey) error {
	query := `
    INSERT INTO api_keys (
        key_id, hashed_key, name, description, user_id, scopes, allowed_ips, allowed_referrers, rate_limit, expires_at
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    RETURNING id, created_at
    `
	err := r.Executor(ctx).QueryRow(ctx, query,
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

// GetAll lists a user's API keys, newest first.
func (r *apiKeyRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]model.APIKey, error) {
	rows, err := r.Executor(ctx).Query(ctx,
		`SELECT `+apiKeyColumns+` FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []model.APIKey
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, *key)
	}
	return keys, rows.Err()
}

// apiKeyColumns and scanAPIKey are shared by the single-key lookups so the
// selected columns and the scan targets cannot drift apart.
const apiKeyColumns = `
	id, key_id, hashed_key, name, description, user_id, scopes, allowed_ips,
	allowed_referrers, rate_limit, created_at, expires_at, last_used_at,
	revoked_at, rotation_enabled, rotated_from_id, rotation_due_at, is_active`

func scanAPIKey(row pgx.Row) (*model.APIKey, error) {
	var key model.APIKey
	err := row.Scan(
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
		&key.CreatedAt,
		&key.ExpiresAt,
		&key.LastUsedAt,
		&key.RevokedAt,
		&key.RotationEnabled,
		&key.RotatedFromID,
		&key.RotationDueAt,
		&key.IsActive,
	)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *apiKeyRepo) GetByKeyID(ctx context.Context, keyID string) (*model.APIKey, error) {
	return scanAPIKey(r.Executor(ctx).QueryRow(ctx,
		`SELECT `+apiKeyColumns+` FROM api_keys WHERE key_id = $1`, keyID))
}

func (r *apiKeyRepo) GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error) {
	return scanAPIKey(r.Executor(ctx).QueryRow(ctx,
		`SELECT `+apiKeyColumns+` FROM api_keys WHERE hashed_key = $1`, keyHash))
}

func (r *apiKeyRepo) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.Executor(ctx).Exec(
		ctx, `
		UPDATE api_keys
		SET last_used_at = NOW()
		WHERE id = $1
		`, id)
	return err
}
