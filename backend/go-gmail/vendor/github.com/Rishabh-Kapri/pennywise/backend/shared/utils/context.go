package utils

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const (
	budgetIDKey      contextKey = "budgetId"
	userIDKey        contextKey = "userId"
	correlationIDKey contextKey = "correlationId"
)

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

func MustUserID(ctx context.Context) uuid.UUID {
	id, err := UserIDFromContext(ctx)
	if err != nil {
		panic("UserIdMiddleware not configured: " + err.Error())
	}
	return id
}

// WithCorrelationID returns a new context with the correlation ID set.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// NewCorrelationID generates a new correlation ID.
func NewCorrelationID() string {
	return uuid.New().String()
}

// CorrelationIDFromContext extracts the correlation ID from the context.
func CorrelationIDFromContext(ctx context.Context) string {
	id, ok := ctx.Value(correlationIDKey).(string)
	if !ok {
		return ""
	}
	return id
}

func GetHeaders(ctx context.Context) []http.Header {
	var headers []http.Header

	// This is an internal service call, add the X-Internal-Service header
	headers = append(headers, http.Header{
		"X-Internal-Service": []string{"true"},
	})

	// Inject correlation ID if available
	if correlationID := CorrelationIDFromContext(ctx); correlationID != "" {
		headers = append(headers, http.Header{
			"X-Correlation-ID": []string{correlationID},
		})
	}

	// Inject budget ID if available
	if budgetID, err := BudgetIDFromContext(ctx); err == nil {
		headers = append(headers, http.Header{
			"X-Budget-ID": []string{budgetID.String()},
		})
	}

	if uid, err := UserIDFromContext(ctx); err == nil {
		headers = append(headers, http.Header{
			"X-User-ID": []string{uid.String()},
		})
	}

	return headers
}
