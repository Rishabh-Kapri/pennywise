package postgres

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type devicePushTokenRepo struct {
	BaseRepository
}

func NewDevicePushTokenRepository(pool *pgxpool.Pool) repository.DevicePushTokenRepository {
	return &devicePushTokenRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *devicePushTokenRepo) Upsert(
	ctx context.Context,
	authUserId uuid.UUID,
	token string,
	platform string,
) (*model.DevicePushToken, error) {
	var t model.DevicePushToken
	err := r.Executor(ctx).QueryRow(
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
	rows, err := r.Executor(ctx).Query(
		ctx,
		`SELECT t.id, t.auth_user_id, t.expo_push_token, t.platform, t.created_at, t.last_seen_at
		 FROM device_push_tokens t
		 JOIN budget_members m ON m.user_id = t.auth_user_id
		 WHERE m.budget_id = $1`,
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
	cmdTag, err := r.Executor(ctx).Exec(
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
