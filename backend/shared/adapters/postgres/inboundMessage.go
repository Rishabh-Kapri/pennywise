package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
)

// inbound_messages: where each bank message stands (migration 00022).

const inboundMessageColumns = `id, budget_id, source, provider, external_id, status, outcome,
	COALESCE(reason, ''), review, transaction_ids, report_job_id, reported_at, created_at, updated_at`

const inboundMessageKeyWhere = `budget_id = $1 AND source = $2 AND provider = $3 AND external_id = $4`

type inboundMessageRepo struct {
	BaseRepository
}

func NewInboundMessageRepository(pool *pgxpool.Pool) repository.InboundMessageRepository {
	return &inboundMessageRepo{BaseRepository: NewBaseRepository(pool)}
}

// RecordReport implements [repository.InboundMessageRepository]. The upsert's
// WHERE is the state machine for reports: an existing row takes a report only
// while processing, and never the same report twice.
func (r *inboundMessageRepo) RecordReport(ctx context.Context, report model.InboundMessageReport) (bool, error) {
	review, err := marshalReview(report.Review)
	if err != nil {
		return false, err
	}
	ids := report.TransactionIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	k := report.Key
	tag, err := r.Executor(ctx).Exec(ctx, `
		INSERT INTO inbound_messages (
			budget_id, source, provider, external_id, status, outcome, reason, review,
			transaction_ids, report_job_id, reported_at
		) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, $11)
		ON CONFLICT (budget_id, source, provider, external_id) DO UPDATE SET
			status = EXCLUDED.status,
			outcome = EXCLUDED.outcome,
			reason = EXCLUDED.reason,
			review = EXCLUDED.review,
			transaction_ids = EXCLUDED.transaction_ids,
			report_job_id = EXCLUDED.report_job_id,
			reported_at = EXCLUDED.reported_at,
			updated_at = now()
		WHERE inbound_messages.status = 'processing'
			AND inbound_messages.report_job_id <> EXCLUDED.report_job_id`,
		k.BudgetID, k.Source, k.Provider, k.ExternalID,
		report.Outcome.StatusAfter(), report.Outcome, report.Reason, review,
		ids, report.ReportJobID, report.ReportedAt,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Get implements [repository.InboundMessageRepository].
func (r *inboundMessageRepo) Get(ctx context.Context, key model.InboundMessageKey) (*model.InboundMessage, error) {
	m, err := scanInboundMessage(r.Executor(ctx).QueryRow(ctx,
		`SELECT `+inboundMessageColumns+` FROM inbound_messages WHERE `+inboundMessageKeyWhere,
		key.BudgetID, key.Source, key.Provider, key.ExternalID))
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, repository.ErrNotFound) {
		return nil, repository.ErrNotFound
	}
	return m, err
}

// Transition implements [repository.InboundMessageRepository].
func (r *inboundMessageRepo) Transition(
	ctx context.Context,
	key model.InboundMessageKey,
	from, to model.InboundMessageStatus,
) (bool, error) {
	tag, err := r.Executor(ctx).Exec(ctx, `
		UPDATE inbound_messages SET status = $6, updated_at = now()
		WHERE `+inboundMessageKeyWhere+` AND status = $5`,
		key.BudgetID, key.Source, key.Provider, key.ExternalID, from, to)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// List implements [repository.InboundMessageRepository].
func (r *inboundMessageRepo) List(
	ctx context.Context,
	budgetID uuid.UUID,
	status model.InboundMessageStatus,
	limit int,
) ([]model.InboundMessage, error) {
	rows, err := r.Executor(ctx).Query(ctx,
		`SELECT `+inboundMessageColumns+` FROM inbound_messages
		WHERE budget_id = $1 AND status = $2
		ORDER BY updated_at DESC, id
		LIMIT $3`, budgetID, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.InboundMessage{}
	for rows.Next() {
		m, err := scanInboundMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// CountByStatus implements [repository.InboundMessageRepository].
func (r *inboundMessageRepo) CountByStatus(
	ctx context.Context,
	budgetID uuid.UUID,
) (map[model.InboundMessageStatus]int, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT status, count(*) FROM inbound_messages WHERE budget_id = $1 GROUP BY status`, budgetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[model.InboundMessageStatus]int{}
	for rows.Next() {
		var status model.InboundMessageStatus
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

func marshalReview(review *model.MessageReview) ([]byte, error) {
	if review == nil {
		return nil, nil
	}
	return json.Marshal(review)
}

func scanInboundMessage(row pgx.Row) (*model.InboundMessage, error) {
	var (
		m      model.InboundMessage
		review []byte
	)
	err := row.Scan(
		&m.ID, &m.BudgetID, &m.Source, &m.Provider, &m.ExternalID, &m.Status, &m.Outcome,
		&m.Reason, &review, &m.TransactionIDs, &m.ReportJobID, &m.ReportedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(review) > 0 {
		m.Review = &model.MessageReview{}
		if err := json.Unmarshal(review, m.Review); err != nil {
			return nil, err
		}
	}
	if m.TransactionIDs == nil {
		m.TransactionIDs = []uuid.UUID{}
	}
	return &m, nil
}
