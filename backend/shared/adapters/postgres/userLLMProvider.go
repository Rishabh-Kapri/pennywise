package postgres

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userLLMProviderRepo struct {
	BaseRepository
}

func NewUserLLMProviderRepository(pool *pgxpool.Pool) repository.UserLLMProviderRepository {
	return &userLLMProviderRepo{BaseRepository: NewBaseRepository(pool)}
}

const userLLMProviderColumns = `
	p.id, p.user_id, p.provider, p.api_key_encrypted, COALESCE(p.api_key_hint, ''),
	COALESCE(p.base_url, ''), p.model, p.priority, p.enabled, p.last_verified_at,
	COALESCE(p.last_error, ''), p.created_at, p.updated_at`

func scanUserLLMProvider(row pgx.Row) (*model.UserLLMProvider, error) {
	var p model.UserLLMProvider
	err := row.Scan(
		&p.ID, &p.UserID, &p.Provider, &p.APIKeyEncrypted, &p.APIKeyHint,
		&p.BaseURL, &p.Model, &p.Priority, &p.Enabled, &p.LastVerifiedAt,
		&p.LastError, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.HasAPIKey = len(p.APIKeyEncrypted) > 0
	return &p, nil
}

func (r *userLLMProviderRepo) list(ctx context.Context, query string, arg any) ([]model.UserLLMProvider, error) {
	rows, err := r.Executor(ctx).Query(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := []model.UserLLMProvider{}
	for rows.Next() {
		p, err := scanUserLLMProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, *p)
	}
	return providers, rows.Err()
}

func (r *userLLMProviderRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.UserLLMProvider, error) {
	providers, err := r.list(ctx, `
		SELECT `+userLLMProviderColumns+`
		FROM user_llm_providers p
		WHERE p.user_id = $1
		ORDER BY p.priority, p.created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	// The API never hands out ciphertext.
	for i := range providers {
		providers[i].APIKeyEncrypted = nil
	}
	return providers, nil
}

func (r *userLLMProviderRepo) ListEnabledForBudget(ctx context.Context, budgetID uuid.UUID) ([]model.UserLLMProvider, error) {
	return r.list(ctx, `
		SELECT `+userLLMProviderColumns+`
		FROM user_llm_providers p
		JOIN budgets b ON b.user_id = p.user_id
		WHERE b.id = $1 AND b.deleted = FALSE AND p.enabled
		ORDER BY p.priority, p.created_at
	`, budgetID)
}

func (r *userLLMProviderRepo) GetForBudget(
	ctx context.Context,
	budgetID uuid.UUID,
	provider model.LLMProviderName,
) (*model.UserLLMProvider, error) {
	p, err := scanUserLLMProvider(r.Executor(ctx).QueryRow(ctx, `
		SELECT `+userLLMProviderColumns+`
		FROM user_llm_providers p
		JOIN budgets b ON b.user_id = p.user_id
		WHERE b.id = $1 AND b.deleted = FALSE AND p.provider = $2
	`, budgetID, provider))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrLLMProviderNotFound
	}
	return p, err
}

func (r *userLLMProviderRepo) Upsert(
	ctx context.Context,
	userID uuid.UUID,
	in repository.UserLLMProviderUpsert,
) (*model.UserLLMProvider, error) {
	// A changed key, URL or model invalidates the last test result.
	p, err := scanUserLLMProvider(r.Executor(ctx).QueryRow(ctx, `
		INSERT INTO user_llm_providers AS p
			(user_id, provider, api_key_encrypted, api_key_hint, base_url, model, priority, enabled)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			api_key_encrypted = CASE WHEN $9 THEN p.api_key_encrypted ELSE EXCLUDED.api_key_encrypted END,
			api_key_hint      = CASE WHEN $9 THEN p.api_key_hint ELSE EXCLUDED.api_key_hint END,
			base_url          = EXCLUDED.base_url,
			model             = EXCLUDED.model,
			priority          = EXCLUDED.priority,
			enabled           = EXCLUDED.enabled,
			last_verified_at  = NULL,
			last_error        = NULL,
			updated_at        = now()
		RETURNING `+userLLMProviderColumns,
		userID, in.Provider, in.APIKeyEncrypted, in.APIKeyHint, in.BaseURL,
		in.Model, in.Priority, in.Enabled, in.KeepAPIKey,
	))
	if err != nil {
		return nil, err
	}
	p.APIKeyEncrypted = nil
	return p, nil
}

func (r *userLLMProviderRepo) Delete(ctx context.Context, userID uuid.UUID, provider model.LLMProviderName) error {
	tag, err := r.Executor(ctx).Exec(ctx,
		`DELETE FROM user_llm_providers WHERE user_id = $1 AND provider = $2`, userID, provider)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrLLMProviderNotFound
	}
	return nil
}

func (r *userLLMProviderRepo) RecordVerification(
	ctx context.Context,
	budgetID uuid.UUID,
	provider model.LLMProviderName,
	testErr string,
) error {
	_, err := r.Executor(ctx).Exec(ctx, `
		UPDATE user_llm_providers p
		SET last_verified_at = CASE WHEN $3 = '' THEN now() ELSE p.last_verified_at END,
		    last_error = NULLIF($3, '')
		FROM budgets b
		WHERE b.id = $1 AND b.user_id = p.user_id AND p.provider = $2
	`, budgetID, provider, testErr)
	return err
}
