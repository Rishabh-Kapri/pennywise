package middleware

import (
	"log/slog"
	"time"

	utils "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/pkg"

	"github.com/gin-gonic/gin"
)

const correlationIDHeader = "X-Correlation-ID"

// RequestLogger is a Gin middleware that:
// 1. Assigns a correlation ID to each request (from header or auto-generated)
// 2. Stores it in the request context for downstream use
// 3. Logs request and response details with the correlation ID
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Extract or generate correlation ID
		correlationID := c.GetHeader(correlationIDHeader)
		if correlationID == "" {
			correlationID = utils.NewCorrelationID()
		}

		// Store in context and set response header
		ctx := utils.WithCorrelationID(c.Request.Context(), correlationID)
		c.Request = c.Request.WithContext(ctx)
		c.Header(correlationIDHeader, correlationID)

		// Process request
		c.Next()

		// Log after request completes
		duration := time.Since(start)
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.String("correlation_id", correlationID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Duration("duration", duration),
			slog.String("client_ip", c.ClientIP()),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		logger := slog.Default()
		logger.LogAttrs(c.Request.Context(), level, "request completed", attrs...)
	}
}
