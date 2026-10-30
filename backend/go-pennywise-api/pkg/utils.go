package utils

import (
	"context"
	"strings"

	"github.com/google/uuid"
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
