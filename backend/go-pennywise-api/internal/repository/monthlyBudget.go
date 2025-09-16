package repository

import (
	"context"
	"fmt"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// type PgxIface interface {
// 	QueryRow(context.Context, string, ...any) pgx.Row
// 	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
// }

type MonthlyBudgetRepository interface {
	GetPgxTx(ctx context.Context) (pgx.Tx, error)
	GetByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string) (*model.MonthlyBudget, error)
	Create(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, monthlyBudget model.MonthlyBudget) error
	UpdateBudgetedByCatIdAndMonth(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, month string, newBudgeted float64) error
	UpdateCarryoverByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string, amount float64) error
}

type monthlyBudgetRepo struct {
	db *pgxpool.Pool
}

func NewMonthlyBudgetRepository(db *pgxpool.Pool) MonthlyBudgetRepository {
	return &monthlyBudgetRepo{db: db}
}

func (r *monthlyBudgetRepo) GetPgxTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.BeginTx(ctx, pgx.TxOptions{})
}

func (r *monthlyBudgetRepo) GetByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string) (*model.MonthlyBudget, error) {
	var mb model.MonthlyBudget
	var err error
	sql := `
    SELECT
			id,
			budget_id,
			category_id,
			month,
			budgeted,
		  carryover_balance
		FROM monthly_budgets
		WHERE category_id = $1 AND budget_id = $2 AND month = $3
	`

	if tx != nil {
		err = tx.QueryRow(ctx, sql, categoryId, budgetId, month).Scan(
			&mb.ID,
			&mb.BudgetID,
			&mb.CategoryID,
			&mb.Month,
			&mb.Budgeted,
			&mb.CarryoverBalance,
		)
	} else {
		err = r.db.QueryRow(
			ctx, sql, categoryId, budgetId, month,
		).Scan(
			&mb.ID,
			&mb.BudgetID,
			&mb.CategoryID,
			&mb.Month,
			&mb.Budgeted,
			&mb.CarryoverBalance,
		)
	}
	if err != nil {
		return nil, err
	}
	return &mb, nil
}

// create a new budget, get the carryover_balance from the previous month
// update carryover_balance for future months
func (r *monthlyBudgetRepo) Create(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, monthlyBudget model.MonthlyBudget) error {
	// 1. find existing monthly budget and throw error if trying to create the same
	cmdTag, err := tx.Exec(
		ctx, `
			SELECT * FROM monthly_budgets
			WHERE budget_id = $1 AND category_id = $2 AND month = $3
		`, budgetId, monthlyBudget.CategoryID, monthlyBudget.Month,
	)
	if cmdTag.RowsAffected() > 0 {
		return fmt.Errorf("monthly budget already exists for month :%v and category: %v", monthlyBudget.Month, monthlyBudget.CategoryID)
	}
	if err != nil {
		return nil
	}

	// 2. get previous month carryover, use default 0
	var prevCarryover float64
	err = tx.QueryRow(ctx, `
		SELECT carryover_balance FROM monthly_budgets
		WHERE budget_id = $1 AND category_id = $2 AND month < $3
		ORDER BY month DESC LIMIT 1
	`, budgetId, monthlyBudget.CategoryID, monthlyBudget.Month).Scan(&prevCarryover)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	initialCarryover := prevCarryover + monthlyBudget.Budgeted + monthlyBudget.CarryoverBalance

	// 3. add new entry
	_, err = tx.Exec(
		ctx, `
			INSERT INTO monthly_budgets (
				budget_id, category_id, budgeted, month, carryover_balance, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		`, budgetId, monthlyBudget.CategoryID, monthlyBudget.Budgeted, monthlyBudget.Month, initialCarryover,
	)
	if err != nil {
		return err
	}

	// 4. Update carryover_balance only for months strictly after the current month
	updateCarryover := monthlyBudget.Budgeted + monthlyBudget.CarryoverBalance
	_, err = tx.Exec(ctx, `
		UPDATE monthly_budgets
		SET carryover_balance = carryover_balance + $1
		WHERE budget_id = $2 AND category_id = $3 AND TO_DATE(month, 'YYYY-MM') > TO_DATE($4, 'YYYY-MM')
	`, updateCarryover, budgetId, monthlyBudget.CategoryID, monthlyBudget.Month)
	if err != nil {
		return err
	}
	return nil
}

// newBudgeted is the new budget amount, the query handles calculating the difference between old and new budgeted
// handles updating carryover_balance for current and future months
func (r *monthlyBudgetRepo) UpdateBudgetedByCatIdAndMonth(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, month string, newBudgeted float64) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Get current budgeted for the month
	var oldBudgeted float64
	err = tx.QueryRow(ctx, `
		SELECT budgeted FROM monthly_budgets
		WHERE budget_id = $1 AND category_id = $2 AND month = $3
	`, budgetId, categoryId, month).Scan(&oldBudgeted)
	if err != nil {
		return err
	}

	diff := newBudgeted - oldBudgeted

	// 2. Update the current month row budgeted and carryover_balance
	cmdTag, err := tx.Exec(
		ctx, `
		UPDATE monthly_budgets SET
		  budgeted = $1,
		  carryover_balance = carryover_balance + $2,
		  updated_at = NOW()
		WHERE budget_id = $3 AND category_id = $4 AND month = $5
		`, newBudgeted, diff, budgetId, categoryId, month,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("Cannot find monthly budget for categoryId %v and month %v: %w", categoryId, month, pgx.ErrNoRows)
	}

	// 3. Update carryover_balance only for months strictly after the current month
	_, err = tx.Exec(ctx, `
		UPDATE monthly_budgets
		SET carryover_balance = carryover_balance + $1
		WHERE budget_id = $2 AND category_id = $3 AND TO_DATE(month, 'YYYY-MM') > TO_DATE($4, 'YYYY-MM')
	`, diff, budgetId, categoryId, month)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// updates the carryover for current and further months
// amount should be the value the carryvover needs to be updated with
func (r *monthlyBudgetRepo) UpdateCarryoverByCatIdAndMonth(
	ctx context.Context,
	tx pgx.Tx,
	budgetId uuid.UUID,
	categoryId uuid.UUID,
	month string,
	amount float64,
) error {
	// 1. find if monthly budget exists, throw error if it doesn't
	var id uuid.UUID
	var oldCarryover float64
	err := tx.QueryRow(ctx, `
		SELECT id, carryover_balance FROM monthly_budgets
		WHERE budget_id = $1 AND category_id = $2 AND month = $3
	`, budgetId, categoryId, month).Scan(&id, &oldCarryover)
	if err != nil {
		return fmt.Errorf("Error finding monthly budget for categoryId %v and month %v: %w", categoryId, month, err)
	}

	if id == uuid.Nil {
		return fmt.Errorf("Cannot find monthly budget for categoryId %v and month %v: %w", categoryId, month, pgx.ErrNoRows)
	}

	cmdTag, err := tx.Exec(ctx, `
		UPDATE monthly_budgets
		SET carryover_balance = carryover_balance + $1
		WHERE TO_DATE(month, 'YYYY-MM') >= TO_DATE($2, 'YYYY-MM') AND category_id = $3
		`, amount, month, categoryId,
	)
	if err != nil {
		return fmt.Errorf("Error updating monthly budget for categoryId %v and month %v: %w", categoryId, month, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("Cannot find monthly budget for categoryId %v and month %v: %w", categoryId, month, pgx.ErrNoRows)
	}
	return nil
}
