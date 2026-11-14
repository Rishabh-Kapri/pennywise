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

// ListByTransaction implements [repository.TransactionSourceRepository].
func (r *transactionSourceRepo) ListByTransaction(
	ctx context.Context,
	budgetID, transactionID uuid.UUID,
) ([]model.TransactionSource, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT id, kind, origin, raw_text, channel, merchant, vpa, ref, account_hint, direction,
		       amount::float8, currency, to_char(occurred_on, 'YYYY-MM-DD'), occurred_at, created_at
		FROM transaction_sources
		WHERE budget_id = $1 AND transaction_id = $2
		ORDER BY created_at, id
	`, budgetID, transactionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := []model.TransactionSource{}
	for rows.Next() {
		var s model.TransactionSource
		if err := rows.Scan(
			&s.ID, &s.Kind, &s.Origin, &s.RawText, &s.Channel, &s.Merchant, &s.VPA, &s.Ref, &s.AccountHint,
			&s.Direction, &s.Amount, &s.Currency, &s.OccurredOn, &s.OccurredAt, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
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

// RefsByTransaction implements [repository.TransactionSourceRepository].
func (r *transactionSourceRepo) RefsByTransaction(
	ctx context.Context,
	budgetID uuid.UUID,
	transactionIDs []uuid.UUID,
) (map[uuid.UUID][]string, error) {
	out := map[uuid.UUID][]string{}
	if len(transactionIDs) == 0 {
		return out, nil
	}
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT transaction_id, ref
		FROM transaction_sources
		WHERE budget_id = $1 AND transaction_id = ANY($2) AND ref IS NOT NULL AND kind <> 'STATEMENT_LINE'
	`, budgetID, transactionIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var ref string
		if err := rows.Scan(&id, &ref); err != nil {
			return nil, err
		}
		out[id] = append(out[id], ref)
	}
	return out, rows.Err()
}

// ReplaceStatementLines implements [repository.TransactionSourceRepository].
// Run it inside a transaction (DBTxn) so a failure leaves the old lines.
func (r *transactionSourceRepo) ReplaceStatementLines(
	ctx context.Context,
	budgetID, importID uuid.UUID,
	lines []model.StatementLineSource,
) error {
	ex := r.Executor(ctx)
	if _, err := ex.Exec(ctx, `
		DELETE FROM transaction_sources
		WHERE budget_id = $1 AND statement_import_id = $2 AND kind = 'STATEMENT_LINE'
	`, budgetID, importID); err != nil {
		return err
	}
	for _, l := range lines {
		if _, err := ex.Exec(ctx, `
			INSERT INTO transaction_sources (
				budget_id, transaction_id, kind, raw_text, origin,
				channel, merchant, vpa, ref, account_hint, direction,
				amount, currency, occurred_on, rewards_points, statement_import_id
			) VALUES (
				$1, $2, 'STATEMENT_LINE', $3, 'PIPELINE',
				NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''),
				$10, NULLIF($11, ''), NULLIF($12, '')::date, $13, $14
			)
			ON CONFLICT (transaction_id, kind) DO UPDATE SET
				raw_text = EXCLUDED.raw_text, origin = 'PIPELINE',
				channel = EXCLUDED.channel, merchant = EXCLUDED.merchant, vpa = EXCLUDED.vpa,
				ref = EXCLUDED.ref, account_hint = EXCLUDED.account_hint, direction = EXCLUDED.direction,
				amount = EXCLUDED.amount, currency = EXCLUDED.currency, occurred_on = EXCLUDED.occurred_on,
				rewards_points = EXCLUDED.rewards_points, statement_import_id = EXCLUDED.statement_import_id,
				updated_at = now()
		`, budgetID, l.TransactionID, l.RawText,
			l.Channel, l.Merchant, l.VPA, l.Ref, l.AccountHint, l.Direction,
			l.Amount, l.Currency, l.OccurredOn, l.RewardsPoints, importID); err != nil {
			return err
		}
	}
	return nil
}
