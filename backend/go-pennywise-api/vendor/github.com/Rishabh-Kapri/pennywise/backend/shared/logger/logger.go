package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
	"github.com/lmittmann/tint"
)

type contextKey string

const correlationIDKey contextKey = "correlationId"

// Setup initializes a structured JSON logger as the default slog logger with the service name.
// It reads the RAILWAY_ENVIRONMENT_NAME environment variable to determine the log level.
func Setup(service string) {
	env := os.Getenv("RAILWAY_ENVIRONMENT_NAME")
	if env == "" {
		env = "local"
	}

	logLevel := logLevelFromEnv(env)
	var handler slog.Handler

	if env == "local" {
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      logLevel,
			AddSource:  logLevel == slog.LevelDebug,
			TimeFormat: "15:04:05",
		})
	} else {

		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: logLevel == slog.LevelDebug,
		})
	}
	slog.SetDefault(slog.New(handler).With("service", service))
}

func logLevelFromEnv(env string) slog.Level {
	switch strings.ToLower(env) {
	case "local", "dev", "development":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
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
	if bid, err := utils.BudgetIDFromContext(ctx); err == nil {
		logger = logger.With("budget_id", bid.String())
	}
	if uid, err := utils.UserIDFromContext(ctx); err == nil {
		logger = logger.With("user_id", uid.String())
	}

	return logger
}
