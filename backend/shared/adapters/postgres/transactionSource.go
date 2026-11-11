package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionSourceRepo struct {
	BaseRepository
}

func NewTransactionSourceRepository(pool *pgxpool.Pool) repository.TransactionSourceRepository {
	return &transactionSourceRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *transactionSourceRepo) RecordFromPipeline(
	ctx context.Context,
	budgetID, transactionID uuid.UUID,
	s model.MessageSource,
) (bool, error) {
	tag, err := r.Executor(ctx).Exec(ctx, `
		INSERT INTO transaction_sources (
			budget_id, transaction_id, kind, raw_text, origin,
			channel, merchant, vpa, ref, account_hint, direction,
			amount, currency, occurred_on, occurred_at
		) VALUES (
			$1, $2, $3, $4, 'PIPELINE',
			NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''),
			NULLIF($11, '')::numeric, NULLIF($12, ''), $13::date, $14
		)
		ON CONFLICT (transaction_id, kind) DO NOTHING
	`, budgetID, transactionID, s.Kind, s.Text,
		s.Channel, s.Merchant, s.VPA, s.Ref, s.AccountHint, s.Direction,
		s.Amount, s.Currency, s.OccurredOn, s.OccurredAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *transactionSourceRepo) LookupKey(
	ctx context.Context,
	budgetID, transactionID uuid.UUID,
) (*model.LookupKey, error) {
	var key model.LookupKey
	err := r.Executor(ctx).QueryRow(ctx, `
		SELECT channel, merchant
		FROM transaction_sources
		WHERE budget_id = $1 AND transaction_id = $2 AND channel IS NOT NULL AND merchant IS NOT NULL
		ORDER BY created_at
		LIMIT 1
	`, budgetID, transactionID).Scan(&key.Channel, &key.Merchant)
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// FindCrossChannel implements [repository.TransactionSourceRepository]. A
// transaction already holding a different message of this kind is left out:
// a channel sends one message per payment.
func (r *transactionSourceRepo) FindCrossChannel(
	ctx context.Context,
	q model.SourceMatchQuery,
) ([]model.SourceMatchCandidate, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT t.id, t.payee_id, t.category_id, t.created_at, s.occurred_at
		FROM transaction_sources s
		JOIN transactions t ON t.id = s.transaction_id AND t.deleted = FALSE
		WHERE s.budget_id = $1 AND t.budget_id = $1 AND t.account_id = $2
		  AND t.amount = round($3::numeric, 2)
		  AND s.kind IN ('EMAIL', 'SMS') AND s.kind <> $4
		  AND s.occurred_on = $5::date
		  AND NOT EXISTS (
			SELECT 1 FROM transaction_sources o
			WHERE o.transaction_id = t.id AND o.kind = $4 AND o.raw_text <> $6
		  )
		ORDER BY t.created_at DESC
		LIMIT 10
	`, q.BudgetID, q.AccountID, q.Amount, q.Kind, q.On.Format(time.DateOnly), q.Text)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var found []model.SourceMatchCandidate
	for rows.Next() {
		var c model.SourceMatchCandidate
		if err := rows.Scan(&c.TransactionID, &c.PayeeID, &c.CategoryID, &c.CreatedAt, &c.OccurredAt); err != nil {
			return nil, err
		}
		found = append(found, c)
	}
	return found, rows.Err()
}
