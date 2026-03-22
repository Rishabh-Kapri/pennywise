package utils

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const budgetIDKey contextKey = "budgetId"

// WithBudgetID returns a new context with the budget ID set.
func WithBudgetID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, budgetIDKey, id)
}

// BudgetIDFromContext extracts the budget ID from the context.
// Returns an error if the budget ID is missing — indicates a middleware misconfiguration.
func BudgetIDFromContext(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctx.Value(budgetIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("budget ID not found in context")
	}
	return id, nil
}

// MustBudgetID extracts the budget ID or panics.
// Only use in code paths guaranteed to run behind BudgetIdMiddleware.
func MustBudgetID(ctx context.Context) uuid.UUID {
	id, err := BudgetIDFromContext(ctx)
	if err != nil {
		panic("BudgetIdMiddleware not configured: " + err.Error())
	}
	return id
}

// @TODO: add more validations
// Returns the month key from date (YYYY-MM-DD) in the format YYYY-MM.
func GetMonthKey(date string) string {
	key := strings.Split(date, "-")
	monthKey := key[0] + "-" + key[1]
	return monthKey
}

func getSortedMonths(values []string) []string {
	sort.Strings(values)
	return values
}

func FillCarryForward(values map[string]float32, month string) map[string]float32 {
	_, exists := values[month]
	if exists {
		return values
	}

	var months []string
	for k := range values {
		months = append(months, k)
	}
	if len(months) == 0 {
		return values
	}
	sortedMonths := getSortedMonths(months)

	values[month] = values[sortedMonths[len(sortedMonths)-1]]

	return values
}

// Updates the carryover_balance column in the monthly_budgets table.
// Pass a context.Context, a pgx.Tx transaction, a categoryId, an amount (reverse it for deletion), and a monthKey (YYYY-MM).
func UpdateCarryover(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, amount float64, monthKey string) error {
	if categoryId == uuid.Nil {
		return fmt.Errorf("categoryId cannot be nil")
	}
	if amount == 0 {
		log.Printf("Skipping carryover update for 0 amount")
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
		log.Printf("Carryover not found for month: %v", monthKey)
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
			log.Printf("No entry found for previous months when creating new monthly_budget, creating with using previous carryover_balance as activity amount")
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

func Float64SliceToVectorString(vec []float64) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%.8f", v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// Helper method to execute a function within a transaction. It will commit if the function returns nil error, otherwise it will rollback.
// This is useful to avoid repeating the same transaction handling code in multiple places. Just pass the function that contains the logic that needs to be executed within the transaction.
// This also ensures that the transaction is properly rolled back in case of any error, preventing potential data inconsistencies.
//
// Example usage:
// err := WithTx(ctx, pool, func(tx pgx.Tx) error {
//     // Your transactional code here, using the provided tx
//     // For example:
//     err := repo.Create(ctx, tx, data)
//     if err != nil {
//         return err
//     }
//     return nil
// })
// if err != nil {
//     // Handle error
// }
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Pass the transaction to the function and execute it
	// This function is passed from the service layer and contains the actual logic that needs to be executed within the transaction
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
