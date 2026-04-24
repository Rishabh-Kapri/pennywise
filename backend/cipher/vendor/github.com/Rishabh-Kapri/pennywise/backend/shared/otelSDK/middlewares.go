package otelSDK

import (
	"log/slog"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/gin-gonic/gin"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.20.0/httpconv"
)

// LogRequest is a Gin middleware that logs the start and completion of each HTTP request
// along with its path. It utilizes the standard slog package.
func (t *Telemetry) LogRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.Info("request", "path", c.Request.URL.Path)
		c.Next()
		slog.Info("request complete", "path", c.Request.URL.Path)
	}
}

// MeterRequestDuration is a Gin middleware that captures the duration of each HTTP request.
// It records the execution time in milliseconds into a predefined OTel Int64Histogram.
// Standard HTTP semantic attributes (like method and route) are automatically attached.
func (t *Telemetry) MeterRequestDuration() gin.HandlerFunc {
	// init metric, here we are using histogram for capturing request duration
	histogram, err := t.MeterInt64Histogram(MetricRequestDurationMillis)
	if err != nil {
		wrappedErr := errs.Wrap(errs.CodeInternalError, "failed to create histogram", err)
		logger.Fatal(wrappedErr.Error())
	}

	return func(c *gin.Context) {
		// capture the start time of the request
		startTime := time.Now()

		// execute next http handler
		c.Next()

		// record the request duration
		duration := time.Since(startTime)
		histogram.Record(
			c.Request.Context(),
			duration.Milliseconds(),
			metric.WithAttributes(
				httpconv.ServerRequest(t.GetServiceName(), c.Request)...,
			),
		)
	}
}

// MeterRequestsInFlight is a Gin middleware that maintains a real-time gauge of active
// concurrent HTTP requests. It increments an Int64UpDownCounter at the start of a request
// and decrements it once the request completes.
func (t *Telemetry) MeterRequestsInFlight() gin.HandlerFunc {
	// init metric, here we are using counter for capturing request in flight
	counter, err := t.MeterInt64UpDownCounter(MetricRequestsInFlight)
	if err != nil {
		wrappedErr := errs.Wrap(errs.CodeInternalError, "failed to create counter", err)
		logger.Fatal(wrappedErr.Error())
	}

	return func(c *gin.Context) {
		// define metric attributes
		attrs := metric.WithAttributes(httpconv.ServerRequest(t.GetServiceName(), c.Request)...)

		// increase the number of requests in flight
		counter.Add(c.Request.Context(), 1, attrs)

		// execute next http handler
		c.Next()

		// decrease the number of requests in flight
		counter.Add(c.Request.Context(), -1, attrs)
	}
}
