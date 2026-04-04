package utils

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
)

const correlationIDKey contextKey = "correlationId"

// SetupLogger initializes a structured JSON logger as the default slog logger.
func SetupLogger() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevelFromEnv(),
	})
	slog.SetDefault(slog.New(handler))
}

func logLevelFromEnv() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("RAILWAY_ENVIRONMENT_NAME"))) {
	case "dev", "development":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

// WithCorrelationID returns a new context with the correlation ID set.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from the context.
func CorrelationIDFromContext(ctx context.Context) string {
	id, ok := ctx.Value(correlationIDKey).(string)
	if !ok {
		return ""
	}
	return id
}

// NewCorrelationID generates a new correlation ID.
func NewCorrelationID() string {
	return uuid.New().String()
}

// Logger returns an slog.Logger enriched with the correlation ID from context.
// Use this in handlers and services to get request-scoped logging.
func Logger(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if cid := CorrelationIDFromContext(ctx); cid != "" {
		logger = logger.With("correlation_id", cid)
	}
	if bid, err := BudgetIDFromContext(ctx); err == nil {
		logger = logger.With("budget_id", bid.String())
	}
	if uid, err := UserIDFromContext(ctx); err == nil {
		logger = logger.With("user_id", uid.String())
	}
	return logger
}
