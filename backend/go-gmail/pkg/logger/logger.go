package logger

import (
	"context"
	"log/slog"
	"os"
)

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
