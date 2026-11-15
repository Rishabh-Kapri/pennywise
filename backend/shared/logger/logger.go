package logger

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

type ContextLogger struct {
	*slog.Logger
	ctx context.Context
}

func (l *ContextLogger) Debug(msg string, args ...any) { l.log(slog.LevelDebug, msg, args...) }
func (l *ContextLogger) Info(msg string, args ...any)  { l.log(slog.LevelInfo, msg, args...) }
func (l *ContextLogger) Warn(msg string, args ...any)  { l.log(slog.LevelWarn, msg, args...) }
func (l *ContextLogger) Error(msg string, args ...any) { l.log(slog.LevelError, msg, args...) }

// log records the caller of Debug/Info/Warn/Error as the source. Going through
// slog.Logger's own methods would report this file instead.
func (l *ContextLogger) log(level slog.Level, msg string, args ...any) {
	if !l.Logger.Enabled(l.ctx, level) {
		return
	}
	writeRecord(l.ctx, l.Logger.Handler(), level, msg, 4, args...)
}

// writeRecord builds a record whose source is skip frames up the stack
// (counting runtime.Callers itself, writeRecord, and the wrappers above it).
func writeRecord(ctx context.Context, h slog.Handler, level slog.Level, msg string, skip int, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(skip, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = h.Handle(ctx, r)
}

func (l *ContextLogger) With(args ...any) *ContextLogger {
	return &ContextLogger{
		Logger: l.Logger.With(args...),
		ctx:    l.ctx,
	}
}

func (l *ContextLogger) WithGroup(name string) *ContextLogger {
	return &ContextLogger{
		Logger: l.Logger.WithGroup(name),
		ctx:    l.ctx,
	}
}

// Setup installs the default slog logger, tagged with the service name. Local
// runs (RAILWAY_ENVIRONMENT_NAME unset or "local") get a colored, human-readable
// console format; set NO_COLOR to drop the colors. Everything else logs JSON.
func Setup(service string) {
	env := os.Getenv("RAILWAY_ENVIRONMENT_NAME")
	if env == "" {
		env = "local"
	}

	logLevel := logLevelFromEnv(env)
	var handler slog.Handler

	if env == "local" {
		handler = newConsoleHandler(os.Stdout, consoleOptions{
			Level:     logLevel,
			Color:     os.Getenv("NO_COLOR") == "",
			AddSource: true,
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
	writeRecord(context.Background(), slog.Default().Handler(), slog.LevelError, msg, 3, args...)
	os.Exit(1)
}

// Logger returns an slog.Logger enriched with the correlation ID from context.
// Use this in handlers and services to get request-scoped logging.
func Logger(ctx context.Context) *ContextLogger {
	logger := slog.Default()
	metadata := utils.RequestMetadataFromContext(ctx)

	if metadata.CorrelationID != "" {
		logger = logger.With("correlation_id", metadata.CorrelationID)
	}
	if metadata.CallerService != "" {
		logger = logger.With("caller_service", metadata.CallerService)
	}
	if metadata.OriginService != "" {
		logger = logger.With("origin_service", metadata.OriginService)
	}
	if metadata.VerifiedInternal {
		logger = logger.With("verified_internal", true)
	}
	if bid, err := utils.BudgetIDFromContext(ctx); err == nil {
		logger = logger.With("budget_id", bid.String())
	}
	if uid, err := utils.UserIDFromContext(ctx); err == nil {
		logger = logger.With("user_id", uid.String())
	}

	return &ContextLogger{Logger: logger, ctx: ctx}
}
