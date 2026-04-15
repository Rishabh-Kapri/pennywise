package utils

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type contextKey string

const budgetIDKey contextKey = "budgetId"
const userIDKey contextKey = "userId"

// WithBudgetID returns a new context with the budget ID set.
func WithBudgetID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, budgetIDKey, id)
}

// BudgetIDFromContext extracts the budget ID from the context.
// Returns an error if the budget ID is missing.
func BudgetIDFromContext(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctx.Value(budgetIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("budget ID not found in context")
	}
	return id, nil
}

// MustBudgetID extracts the budget ID or panics.
func MustBudgetID(ctx context.Context) uuid.UUID {
	id, err := BudgetIDFromContext(ctx)
	if err != nil {
		panic("BudgetIdMiddleware not configured: " + err.Error())
	}
	return id
}

// WithUserID returns a new context with the authenticated user's ID set.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserIDFromContext extracts the authenticated user's ID from the context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("user ID not found in context")
	}
	return id, nil
}
