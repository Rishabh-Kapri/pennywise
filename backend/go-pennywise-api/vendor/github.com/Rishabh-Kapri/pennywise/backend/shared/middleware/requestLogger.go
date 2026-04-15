package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
)

const (
	correlationIDHeader = "X-Correlation-ID"
	maxLoggedBodyBytes  = 8 * 1024
)

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseBodyWriter) Write(data []byte) (int, error) {
	_, _ = w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *responseBodyWriter) WriteString(s string) (int, error) {
	_, _ = w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// RequestLogger is a Gin middleware that logs request and response details.
// It also generates/extracts a Correlation ID.
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

		logger := logger.Logger(ctx)
		debugLogging := logger.Enabled(c.Request.Context(), slog.LevelDebug)

		var requestBody string
		var responseWriter *responseBodyWriter
		// If debug logging is enabled, capture request body and response body
		if debugLogging {
			requestBody = captureRequestBody(c)
			responseWriter = &responseBodyWriter{
				ResponseWriter: c.Writer,
				body:           bytes.NewBuffer(nil),
			}
			c.Writer = responseWriter
		}

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

		if debugLogging {
			if requestBody != "" {
				attrs = append(attrs, slog.String("request_body", requestBody))
			}
			if responseWriter != nil && responseWriter.body.Len() > 0 {
				attrs = append(attrs, slog.String("response_body", truncateBody(responseWriter.body.Bytes())))
			}
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		logger.LogAttrs(c.Request.Context(), level, "request completed", attrs...)
	}
}

func captureRequestBody(c *gin.Context) string {
	if c.Request == nil || c.Request.Body == nil {
		return ""
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "<error reading body: " + err.Error() + ">"
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	return truncateBody(body)
}

func truncateBody(body []byte) string {
	if len(body) <= maxLoggedBodyBytes {
		return string(body)
	}

	return string(body[:maxLoggedBodyBytes]) + "...(truncated)"
}
