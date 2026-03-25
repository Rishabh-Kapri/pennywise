package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
)

type contextKey string

const correlationIDKey contextKey = "correlationId"

// Setup initializes the default slog logger with a JSON handler writing to stdout.
// Info-level logs go to stdout (Railway treats as info), errors go to stderr (Railway treats as error).
func Setup() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
}

// Fatal logs at error level and exits with code 1.
// Use this as a replacement for log.Fatalf.
func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

// FatalContext logs at error level with context and exits with code 1.
func FatalContext(ctx context.Context, msg string, args ...any) {
	slog.ErrorContext(ctx, msg, args...)
	os.Exit(1)
}

// NewCorrelationID generates a new correlation ID.
func NewCorrelationID() string {
	return uuid.New().String()
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

// Logger returns an slog.Logger enriched with the correlation ID from context.
func Logger(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if cid := CorrelationIDFromContext(ctx); cid != "" {
		logger = logger.With("correlation_id", cid)
	}
	return logger
}
