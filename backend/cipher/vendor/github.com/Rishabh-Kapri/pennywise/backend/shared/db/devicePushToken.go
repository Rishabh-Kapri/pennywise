package db

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DevicePushTokenRepository interface {
	BaseRepositoryInterface
	// Upsert registers a token for a user, refreshing last_seen_at (and owner)
	// when the token is already known.
	Upsert(ctx context.Context, authUserId uuid.UUID, token string, platform string) (*model.DevicePushToken, error)
	// GetByBudgetId returns all tokens belonging to the owner of a budget.
	GetByBudgetId(ctx context.Context, budgetId uuid.UUID) ([]model.DevicePushToken, error)
	DeleteByToken(ctx context.Context, authUserId uuid.UUID, token string) error
}

type devicePushTokenRepo struct {
	BaseRepository
}

func NewDevicePushTokenRepository(pool *pgxpool.Pool) DevicePushTokenRepository {
	return &devicePushTokenRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *devicePushTokenRepo) Upsert(
	ctx context.Context,
	authUserId uuid.UUID,
	token string,
	platform string,
) (*model.DevicePushToken, error) {
	var t model.DevicePushToken
	err := r.Executor(nil).QueryRow(
		ctx,
		`INSERT INTO device_push_tokens (auth_user_id, expo_push_token, platform)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (expo_push_token)
		 DO UPDATE SET auth_user_id = EXCLUDED.auth_user_id, platform = EXCLUDED.platform, last_seen_at = NOW()
		 RETURNING id, auth_user_id, expo_push_token, platform, created_at, last_seen_at`,
		authUserId, token, platform,
	).Scan(&t.ID, &t.AuthUserID, &t.ExpoPushToken, &t.Platform, &t.CreatedAt, &t.LastSeenAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *devicePushTokenRepo) GetByBudgetId(
	ctx context.Context,
	budgetId uuid.UUID,
) ([]model.DevicePushToken, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT t.id, t.auth_user_id, t.expo_push_token, t.platform, t.created_at, t.last_seen_at
		 FROM device_push_tokens t
		 JOIN budgets b ON b.user_id = t.auth_user_id
		 WHERE b.id = $1`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []model.DevicePushToken
	for rows.Next() {
		var t model.DevicePushToken
		if err := rows.Scan(&t.ID, &t.AuthUserID, &t.ExpoPushToken, &t.Platform, &t.CreatedAt, &t.LastSeenAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (r *devicePushTokenRepo) DeleteByToken(ctx context.Context, authUserId uuid.UUID, token string) error {
	cmdTag, err := r.Executor(nil).Exec(
		ctx,
		`DELETE FROM device_push_tokens WHERE auth_user_id = $1 AND expo_push_token = $2`,
		authUserId, token,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("Push token not found")
	}
	return nil
}
