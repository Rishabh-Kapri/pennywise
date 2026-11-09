package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userLLMModelRepo struct {
	BaseRepository
}

func NewUserLLMModelRepository(pool *pgxpool.Pool) repository.UserLLMModelRepository {
	return &userLLMModelRepo{BaseRepository: NewBaseRepository(pool)}
}

const userLLMModelColumns = `
	m.id, m.user_id, m.provider_id, p.provider, m.kind, m.slug, COALESCE(m.endpoint, ''),
	m.is_default, m.enabled, m.priority, m.last_verified_at, COALESCE(m.last_error, ''), m.created_at, m.updated_at`

func scanUserLLMModel(row pgx.Row, extra ...any) (*model.UserLLMModel, error) {
	var m model.UserLLMModel
	dest := []any{
		&m.ID, &m.UserID, &m.ProviderID, &m.Provider, &m.Kind, &m.Slug, &m.Endpoint,
		&m.IsDefault, &m.Enabled, &m.Priority, &m.LastVerifiedAt, &m.LastError, &m.CreatedAt, &m.UpdatedAt,
	}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *userLLMModelRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.UserLLMModel, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT `+userLLMModelColumns+`
		FROM user_llm_models m
		JOIN user_llm_providers p ON p.id = m.provider_id
		WHERE m.user_id = $1
		ORDER BY m.kind, m.is_default DESC, m.priority, m.created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := []model.UserLLMModel{}
	for rows.Next() {
		m, err := scanUserLLMModel(rows)
		if err != nil {
			return nil, err
		}
		models = append(models, *m)
	}
	return models, rows.Err()
}

func (r *userLLMModelRepo) ListUsableForBudget(
	ctx context.Context,
	budgetID uuid.UUID,
	kinds ...model.LLMModelKind,
) ([]model.UserLLMModel, error) {
	kindNames := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		kindNames = append(kindNames, string(kind))
	}
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT `+userLLMModelColumns+`, p.api_key_encrypted, COALESCE(p.base_url, ''), p.updated_at
		FROM user_llm_models m
		JOIN user_llm_providers p ON p.id = m.provider_id
		JOIN budgets b ON b.user_id = m.user_id
		WHERE b.id = $1 AND b.deleted = FALSE AND p.enabled AND m.enabled
			AND (cardinality($2::text[]) = 0 OR m.kind::text = ANY($2::text[]))
		ORDER BY m.is_default DESC, m.priority, m.created_at
	`, budgetID, kindNames)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := []model.UserLLMModel{}
	for rows.Next() {
		var (
			key       []byte
			baseURL   string
			keyUpdate time.Time
		)
		m, err := scanUserLLMModel(rows, &key, &baseURL, &keyUpdate)
		if err != nil {
			return nil, err
		}
		m.APIKeyEncrypted, m.BaseURL, m.KeyUpdatedAt = key, baseURL, keyUpdate
		models = append(models, *m)
	}
	return models, rows.Err()
}

func (r *userLLMModelRepo) Get(ctx context.Context, userID, id uuid.UUID) (*model.UserLLMModel, error) {
	m, err := scanUserLLMModel(r.Executor(ctx).QueryRow(ctx, `
		SELECT `+userLLMModelColumns+`
		FROM user_llm_models m
		JOIN user_llm_providers p ON p.id = m.provider_id
		WHERE m.user_id = $1 AND m.id = $2
	`, userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrLLMModelNotFound
	}
	return m, err
}

func (r *userLLMModelRepo) Create(
	ctx context.Context,
	userID uuid.UUID,
	in repository.UserLLMModelCreate,
) (*model.UserLLMModel, error) {
	var id uuid.UUID
	err := r.InTx(ctx, func(ctx context.Context) error {
		db := r.Executor(ctx)
		if in.MakeDefault {
			if _, err := db.Exec(ctx, `
				UPDATE user_llm_models SET is_default = false, updated_at = now()
				WHERE user_id = $1 AND kind = $2 AND is_default
			`, userID, in.Kind); err != nil {
				return err
			}
		}
		// The key must be the user's own; the first model of a kind becomes
		// its default.
		err := db.QueryRow(ctx, `
			INSERT INTO user_llm_models (user_id, provider_id, kind, slug, endpoint, is_default, last_verified_at, priority)
			SELECT $1, p.id, $3, $4, NULLIF($5, ''),
				$6 OR NOT EXISTS (
					SELECT 1 FROM user_llm_models d WHERE d.user_id = $1 AND d.kind = $3 AND d.is_default
				),
				CASE WHEN $7 THEN now() END,
				(SELECT COALESCE(MAX(o.priority) + 1, 0) FROM user_llm_models o WHERE o.user_id = $1 AND o.kind = $3)
			FROM user_llm_providers p
			WHERE p.user_id = $1 AND p.provider = $2
			RETURNING id
		`, userID, in.Provider, in.Kind, in.Slug, in.Endpoint, in.MakeDefault, in.VerifiedAt).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.ErrLLMProviderNotFound
		}
		return mapModelUniqueViolation(err)
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, userID, id)
}

func (r *userLLMModelRepo) Update(
	ctx context.Context,
	userID, id uuid.UUID,
	slug, endpoint string,
) (*model.UserLLMModel, error) {
	tag, err := r.Executor(ctx).Exec(ctx, `
		UPDATE user_llm_models
		SET slug = $3, endpoint = NULLIF($4, ''), last_verified_at = now(), last_error = NULL, updated_at = now()
		WHERE user_id = $1 AND id = $2
	`, userID, id, slug, endpoint)
	if err := mapModelUniqueViolation(err); err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, repository.ErrLLMModelNotFound
	}
	return r.Get(ctx, userID, id)
}

func (r *userLLMModelRepo) SetDefault(ctx context.Context, userID, id uuid.UUID) (*model.UserLLMModel, error) {
	err := r.InTx(ctx, func(ctx context.Context) error {
		db := r.Executor(ctx)
		// Clear the old default first: the partial unique index allows one.
		if _, err := db.Exec(ctx, `
			UPDATE user_llm_models SET is_default = false, updated_at = now()
			WHERE user_id = $1 AND is_default AND id <> $2
				AND kind = (SELECT kind FROM user_llm_models WHERE user_id = $1 AND id = $2)
		`, userID, id); err != nil {
			return err
		}
		tag, err := db.Exec(ctx, `
			UPDATE user_llm_models SET is_default = true, updated_at = now()
			WHERE user_id = $1 AND id = $2
		`, userID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return repository.ErrLLMModelNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, userID, id)
}

func (r *userLLMModelRepo) SetEnabled(
	ctx context.Context,
	userID, id uuid.UUID,
	enabled bool,
) (*model.UserLLMModel, error) {
	tag, err := r.Executor(ctx).Exec(ctx, `
		UPDATE user_llm_models SET enabled = $3, updated_at = now()
		WHERE user_id = $1 AND id = $2
	`, userID, id, enabled)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, repository.ErrLLMModelNotFound
	}
	return r.Get(ctx, userID, id)
}

func (r *userLLMModelRepo) Reorder(
	ctx context.Context,
	userID uuid.UUID,
	kind model.LLMModelKind,
	ids []uuid.UUID,
) error {
	return r.InTx(ctx, func(ctx context.Context) error {
		db := r.Executor(ctx)
		var total int
		if err := db.QueryRow(ctx, `
			SELECT count(*) FROM user_llm_models WHERE user_id = $1 AND kind = $2
		`, userID, kind).Scan(&total); err != nil {
			return err
		}
		tag, err := db.Exec(ctx, `
			UPDATE user_llm_models m
			SET priority = ordered.position - 1, updated_at = now()
			FROM unnest($3::uuid[]) WITH ORDINALITY AS ordered(id, position)
			WHERE m.id = ordered.id AND m.user_id = $1 AND m.kind = $2
		`, userID, kind, ids)
		if err != nil {
			return err
		}
		// Returning an error rolls the update back.
		if int(tag.RowsAffected()) != total || len(ids) != total {
			return repository.ErrLLMModelNotFound
		}
		return nil
	})
}

func (r *userLLMModelRepo) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return r.InTx(ctx, func(ctx context.Context) error {
		db := r.Executor(ctx)
		tag, err := db.Exec(ctx, `DELETE FROM user_llm_models WHERE user_id = $1 AND id = $2`, userID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return repository.ErrLLMModelNotFound
		}
		return promoteDefaults(ctx, db, userID)
	})
}

func (r *userLLMModelRepo) RecordVerification(ctx context.Context, budgetID, id uuid.UUID, testErr string) error {
	_, err := r.Executor(ctx).Exec(ctx, `
		UPDATE user_llm_models m
		SET last_verified_at = CASE WHEN $3 = '' THEN now() ELSE m.last_verified_at END,
		    last_error = NULLIF($3, '')
		FROM budgets b
		WHERE b.id = $1 AND b.user_id = m.user_id AND m.id = $2
	`, budgetID, id, testErr)
	return err
}

// promoteDefaults gives every kind that has models but no default its first
// model by priority as the default, preferring enabled ones. Run after deleting models.
func promoteDefaults(ctx context.Context, db DBTX, userID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE user_llm_models m SET is_default = true, updated_at = now()
		FROM (
			SELECT DISTINCT ON (kind) id
			FROM user_llm_models
			WHERE user_id = $1
			ORDER BY kind, NOT enabled, priority, created_at
		) oldest
		WHERE m.id = oldest.id
			AND NOT EXISTS (
				SELECT 1 FROM user_llm_models d WHERE d.user_id = $1 AND d.kind = m.kind AND d.is_default
			)
	`, userID)
	return err
}

func mapModelUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName != "idx_user_llm_models_default" {
		return repository.ErrLLMModelExists
	}
	return err
}
