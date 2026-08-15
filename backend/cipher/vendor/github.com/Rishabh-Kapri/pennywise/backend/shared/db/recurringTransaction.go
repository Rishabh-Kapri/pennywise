package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RecurringTransactionRepository interface {
	BaseRepositoryInterface
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.RecurringTransaction, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.RecurringTransaction, error)
	// GetDue returns rules ready to fire on or before today, across every
	// budget, so the background scheduler can run a single scan.
	GetDue(ctx context.Context, today string, budgetId *uuid.UUID) ([]model.RecurringTransaction, error)
	Create(ctx context.Context, budgetId uuid.UUID, rule model.RecurringTransaction) (*model.RecurringTransaction, error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, rule model.RecurringTransaction) error
	MarkRun(ctx context.Context, tx pgx.Tx, id uuid.UUID, nextDate model.Date, lastRunDate model.Date) error
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}

type recurringTransactionRepo struct {
	BaseRepository
}

func NewRecurringTransactionRepository(pool *pgxpool.Pool) RecurringTransactionRepository {
	return &recurringTransactionRepo{BaseRepository: NewBaseRepository(pool)}
}

const recurringSelect = `
	SELECT
		r.id, r.budget_id, r.name, r.account_id, r.payee_id, r.category_id,
		r.amount, COALESCE(r.note, ''), r.frequency, r.interval_count,
		r.next_date, r.end_date, r.last_run_date,
		r.paused, r.deleted, r.created_at, r.updated_at,
		a.name, p.name, c.name
	FROM recurring_transactions r
	JOIN accounts a ON a.id = r.account_id
	LEFT JOIN payees p ON p.id = r.payee_id
	LEFT JOIN categories c ON c.id = r.category_id
`

func scanRecurring(rows pgx.Rows) (model.RecurringTransaction, error) {
	var r model.RecurringTransaction
	err := rows.Scan(
		&r.ID,
		&r.BudgetID,
		&r.Name,
		&r.AccountID,
		&r.PayeeID,
		&r.CategoryID,
		&r.Amount,
		&r.Note,
		&r.Frequency,
		&r.IntervalCount,
		&r.NextDate,
		&r.EndDate,
		&r.LastRunDate,
		&r.Paused,
		&r.Deleted,
		&r.CreatedAt,
		&r.UpdatedAt,
		&r.AccountName,
		&r.PayeeName,
		&r.CategoryName,
	)
	return r, err
}

func (r *recurringTransactionRepo) GetAll(
	ctx context.Context,
	budgetId uuid.UUID,
) ([]model.RecurringTransaction, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		recurringSelect+` WHERE r.budget_id = $1 AND r.deleted = FALSE ORDER BY r.next_date ASC, r.name ASC`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]model.RecurringTransaction, 0)
	for rows.Next() {
		rule, err := scanRecurring(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rule)
	}
	return results, rows.Err()
}

func (r *recurringTransactionRepo) GetById(
	ctx context.Context,
	budgetId uuid.UUID,
	id uuid.UUID,
) (*model.RecurringTransaction, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		recurringSelect+` WHERE r.budget_id = $1 AND r.id = $2 AND r.deleted = FALSE`,
		budgetId,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, pgx.ErrNoRows
	}
	rule, err := scanRecurring(rows)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *recurringTransactionRepo) GetDue(
	ctx context.Context,
	today string,
	budgetId *uuid.UUID,
) ([]model.RecurringTransaction, error) {
	query := recurringSelect + `
		WHERE r.deleted = FALSE AND r.paused = FALSE
			AND r.next_date <= $1
			AND (r.end_date IS NULL OR r.next_date <= r.end_date)`
	args := []any{today}
	if budgetId != nil {
		args = append(args, *budgetId)
		query += ` AND r.budget_id = $2`
	}
	query += ` ORDER BY r.next_date ASC`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]model.RecurringTransaction, 0)
	for rows.Next() {
		rule, err := scanRecurring(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rule)
	}
	return results, rows.Err()
}

func (r *recurringTransactionRepo) Create(
	ctx context.Context,
	budgetId uuid.UUID,
	rule model.RecurringTransaction,
) (*model.RecurringTransaction, error) {
	var id uuid.UUID
	err := r.Executor(nil).QueryRow(
		ctx,
		`INSERT INTO recurring_transactions (
			budget_id, name, account_id, payee_id, category_id, amount, note,
			frequency, interval_count, next_date, end_date, paused
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`,
		budgetId,
		rule.Name,
		rule.AccountID,
		rule.PayeeID,
		rule.CategoryID,
		rule.Amount,
		rule.Note,
		rule.Frequency,
		rule.IntervalCount,
		rule.NextDate,
		rule.EndDate,
		rule.Paused,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetById(ctx, budgetId, id)
}

func (r *recurringTransactionRepo) Update(
	ctx context.Context,
	budgetId uuid.UUID,
	id uuid.UUID,
	rule model.RecurringTransaction,
) error {
	cmdTag, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE recurring_transactions SET
			name = $1,
			account_id = $2,
			payee_id = $3,
			category_id = $4,
			amount = $5,
			note = $6,
			frequency = $7,
			interval_count = $8,
			next_date = $9,
			end_date = $10,
			paused = $11,
			updated_at = NOW()
		WHERE budget_id = $12 AND id = $13 AND deleted = FALSE`,
		rule.Name,
		rule.AccountID,
		rule.PayeeID,
		rule.CategoryID,
		rule.Amount,
		rule.Note,
		rule.Frequency,
		rule.IntervalCount,
		rule.NextDate,
		rule.EndDate,
		rule.Paused,
		budgetId,
		id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *recurringTransactionRepo) MarkRun(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
	nextDate model.Date,
	lastRunDate model.Date,
) error {
	_, err := r.Executor(tx).Exec(
		ctx,
		`UPDATE recurring_transactions
		 SET next_date = $1, last_run_date = $2, updated_at = NOW()
		 WHERE id = $3`,
		nextDate,
		lastRunDate,
		id,
	)
	return err
}

func (r *recurringTransactionRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	cmdTag, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE recurring_transactions SET deleted = TRUE, updated_at = NOW()
		 WHERE budget_id = $1 AND id = $2`,
		budgetId,
		id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
