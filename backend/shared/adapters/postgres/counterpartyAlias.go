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

type counterpartyAliasRepo struct {
	BaseRepository
}

func NewCounterpartyAliasRepository(pool *pgxpool.Pool) repository.CounterpartyAliasRepository {
	return &counterpartyAliasRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *counterpartyAliasRepo) FindMatch(
	ctx context.Context,
	budgetID uuid.UUID,
	channel, merchant string,
) (*model.CounterpartyAliasMatch, error) {
	var match model.CounterpartyAliasMatch
	err := r.Executor(ctx).QueryRow(ctx, `
		SELECT p.name, c.name
		FROM counterparty_aliases a
		JOIN payees p ON p.id = a.payee_id AND p.deleted = FALSE
		JOIN categories c ON c.id = a.category_id AND c.deleted = FALSE
		WHERE a.budget_id = $1 AND a.channel = $2 AND a.merchant = $3
	`, budgetID, channel, merchant).Scan(&match.PayeeName, &match.CategoryName)
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &match, nil
}

func (r *counterpartyAliasRepo) Confirm(ctx context.Context, in model.CounterpartyAliasConfirm) error {
	_, err := r.Executor(ctx).Exec(ctx, `
		INSERT INTO counterparty_aliases
			(budget_id, channel, merchant, payee_id, category_id, source, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, 'MANUAL', now())
		ON CONFLICT (budget_id, channel, merchant) DO UPDATE SET
			payee_id = EXCLUDED.payee_id,
			category_id = EXCLUDED.category_id,
			source = 'MANUAL',
			updated_at = now()
	`, in.BudgetID, in.Channel, in.Merchant, in.PayeeID, in.CategoryID)
	return err
}

func (r *counterpartyAliasRepo) Learn(ctx context.Context, in model.CounterpartyAliasLearn) error {
	_, err := r.Executor(ctx).Exec(ctx, `
		INSERT INTO counterparty_aliases
			(budget_id, channel, merchant, payee_id, category_id, source, hit_count, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, 'AUTO_LEARNED', 1, now())
		ON CONFLICT (budget_id, channel, merchant) DO UPDATE SET
			hit_count = counterparty_aliases.hit_count + 1,
			last_seen_at = now(),
			category_id = COALESCE(counterparty_aliases.category_id, EXCLUDED.category_id),
			updated_at = now()
	`, in.BudgetID, in.Channel, in.Merchant, in.PayeeID, in.CategoryID)
	return err
}
