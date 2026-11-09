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
	COALESCE(p.base_url, ''), p.enabled, p.created_at, p.updated_at`

func scanUserLLMProvider(row pgx.Row) (*model.UserLLMProvider, error) {
	var p model.UserLLMProvider
	err := row.Scan(
		&p.ID, &p.UserID, &p.Provider, &p.APIKeyEncrypted, &p.APIKeyHint,
		&p.BaseURL, &p.Enabled, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.HasAPIKey = len(p.APIKeyEncrypted) > 0
	return &p, nil
}

func (r *userLLMProviderRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.UserLLMProvider, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT `+userLLMProviderColumns+`
		FROM user_llm_providers p
		WHERE p.user_id = $1
		ORDER BY p.created_at
	`, userID)
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
		// The API never hands out ciphertext.
		p.APIKeyEncrypted = nil
		providers = append(providers, *p)
	}
	return providers, rows.Err()
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
	p, err := scanUserLLMProvider(r.Executor(ctx).QueryRow(ctx, `
		INSERT INTO user_llm_providers AS p
			(user_id, provider, api_key_encrypted, api_key_hint, base_url, enabled)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			api_key_encrypted = CASE WHEN $7 THEN p.api_key_encrypted ELSE EXCLUDED.api_key_encrypted END,
			api_key_hint      = CASE WHEN $7 THEN p.api_key_hint ELSE EXCLUDED.api_key_hint END,
			base_url          = EXCLUDED.base_url,
			enabled           = EXCLUDED.enabled,
			updated_at        = now()
		RETURNING `+userLLMProviderColumns,
		userID, in.Provider, in.APIKeyEncrypted, in.APIKeyHint, in.BaseURL, in.Enabled, in.KeepAPIKey,
	))
	if err != nil {
		return nil, err
	}
	p.APIKeyEncrypted = nil
	return p, nil
}

func (r *userLLMProviderRepo) Delete(ctx context.Context, userID uuid.UUID, provider model.LLMProviderName) error {
	return r.InTx(ctx, func(ctx context.Context) error {
		// Deleting the key cascades to its models; any kind that loses its
		// default gets the oldest remaining model as the new one.
		tag, err := r.Executor(ctx).Exec(ctx,
			`DELETE FROM user_llm_providers WHERE user_id = $1 AND provider = $2`, userID, provider)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return repository.ErrLLMProviderNotFound
		}
		return promoteDefaults(ctx, r.Executor(ctx), userID)
	})
}
