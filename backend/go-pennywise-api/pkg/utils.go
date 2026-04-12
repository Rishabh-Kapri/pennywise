package utils

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type contextKey string

const (
	budgetIDKey contextKey = "budgetId"
	userIDKey   contextKey = "authUserID"
)

// WithBudgetID returns a new context with the budget ID set.
func WithBudgetID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, budgetIDKey, id)
}

// @TODO: add more validations
// Returns the month key from date (YYYY-MM-DD) in the format YYYY-MM.
func GetMonthKey(date string) string {
	key := strings.Split(date, "-")
	monthKey := key[0] + "-" + key[1]
	return monthKey
}

// Updates the carryover_balance column in the monthly_budgets table.
// Pass a context.Context, a pgx.Tx transaction, a categoryId, an amount (reverse it for deletion), and a monthKey (YYYY-MM).
func UpdateCarryover(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, amount float64, monthKey string) error {
	if categoryId == uuid.Nil {
		return fmt.Errorf("categoryId cannot be nil")
	}
	if amount == 0 {
		logger.Logger(ctx).Info("skipping carryover update for 0 amount")
		return nil
	}
	if monthKey == "" {
		return fmt.Errorf("monthKey cannot be empty")
	}
	cmdTag, err := tx.Exec(
		ctx, `
			UPDATE monthly_budgets
			SET carryover_balance = carryover_balance + $1
			WHERE TO_DATE(month, 'YYYY-MM') >= TO_DATE($2, 'YYYY-MM') AND category_id = $3
		`, amount, monthKey, categoryId,
	)
	if err != nil {
		return fmt.Errorf("failed to update carryover: %v", err)
	}
	if cmdTag.RowsAffected() == 0 {
		budgeted := 0.0
		// @TODO: see if carryover_balance fetching can be done through pure sql
		logger.Logger(ctx).Info("carryover not found for month, creating", "month", monthKey)
		newCmdTag, err := tx.Exec(
			ctx, `
			INSERT INTO monthly_budgets (
			  budget_id, category_id, budgeted, month, carryover_balance, created_at, updated_at
			) VALUES (
			  $1, $2, $3, $4,
			  COALESCE(
			    (
			      SELECT carryover_balance + $3 + $5
			      FROM monthly_budgets
			      WHERE budget_id = $1 AND category_id = $2 AND month < $4
			      ORDER BY month DESC
			      LIMIT 1
			    ),
			    $3 - $5
			  ),
		    NOW(), NOW()
			)
			`, budgetId, categoryId, budgeted, monthKey, amount,
		)
		if err != nil {
			return err
		}
		if newCmdTag.RowsAffected() == 0 {
			logger.Logger(ctx).Info("no previous months found, creating with activity amount as carryover")
			_, err := tx.Exec(
				ctx, `
				INSERT INTO monthly_budgets (
				  budget_id, category_id, budgeted, month, carryover_balance, created_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
				`, budgetId, categoryId, budgeted, monthKey, amount,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
