package otelSDK

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// TelemetryProvider defines the contract for telemetry operations.
// It is designed to allow easy mocking in tests, though in practice, OTel
// itself acts as the primary abstraction layer. Functions here expose basic
// instrument creation, span generation, and Gin middleware handlers.
type TelemetryProvider interface {
	GetServiceName() string
	MeterInt64Histogram(metric Metric) (otelmetric.Int64Histogram, error)
	MeterInt64UpDownCounter(metric Metric) (otelmetric.Int64UpDownCounter, error)
	TraceStart(ctx context.Context, name string) (context.Context, oteltrace.Span)
	LogRequest() gin.HandlerFunc
	MeterRequestDuration() gin.HandlerFunc
	MeterRequestsInFlight() gin.HandlerFunc
	Shutdown(ctx context.Context) error
}

// Telemetry serves as a wrapper around the OpenTelemetry providers, meter, and tracer.
// Design Note: This struct aggregates the core OTel SDK components. While it abstracts
// provider creation, it intentionally returns OTel-native types (like oteltrace.Span)
// to callers, allowing them to leverage standard OTel APIs seamlessly.
type Telemetry struct {
	lp     *log.LoggerProvider
	mp     *metric.MeterProvider
	tp     *trace.TracerProvider
	meter  otelmetric.Meter
	Tracer oteltrace.Tracer
	cfg    Config
}

// NewTelemetry initializes a new Telemetry instance, setting up the global OpenTelemetry
// providers (Tracer, Meter, Logger) and the context propagator. It returns a fully configured
// Telemetry struct or an error if provider initialization fails.
func NewTelemetry(ctx context.Context, cfg Config) (*Telemetry, error) {
	slog.Info("telemetry init", "cfg", cfg)
	rp := newResource(cfg.ServiceName, cfg.ServiceVersion, cfg.Environment)

	// Set up propagator for cross-service trace context (W3C traceparent + baggage headers)
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	var lp *log.LoggerProvider
	var mp *metric.MeterProvider
	var tp *trace.TracerProvider
	var meter otelmetric.Meter
	var tracer oteltrace.Tracer
	var err error

	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Fatal("otel error", "error", err)
	}))
	// Always initialize providers. The providers themselves check OTEL_*_EXPORTER
	// environment variables to determine which exporters (if any) to attach.
	lp, err = newLoggerProvider(ctx, cfg, rp)
	if err != nil {
		return nil, err
	}

	mp, err = newMeterProvider(ctx, cfg, rp)
	if err != nil {
		return nil, err
	}
	meter = mp.Meter(cfg.ServiceName)

	tp, err = newTracerProvider(ctx, cfg, rp)
	if err != nil {
		return nil, err
	}
	tracer = tp.Tracer(cfg.ServiceName)

	return &Telemetry{
		lp:     lp,
		mp:     mp,
		tp:     tp,
		meter:  meter,
		Tracer: tracer,
		cfg:    cfg,
	}, nil
}

// GetServiceName returns the configured name of the service, derived from Config.
func (t *Telemetry) GetServiceName() string {
	return t.cfg.ServiceName
}

// MeterInt64Histogram creates or retrieves an Int64Histogram instrument from the underlying
// OTel meter. Histograms are ideal for recording value distributions, such as request latencies.
func (t *Telemetry) MeterInt64Histogram(metric Metric) (otelmetric.Int64Histogram, error) {
	histogram, err := t.meter.Int64Histogram(
		metric.Name,
		otelmetric.WithDescription(metric.Description),
		otelmetric.WithUnit(metric.Unit),
	)
	if err != nil {
		return nil, err
	}

	return histogram, nil
}

// MeterInt64UpDownCounter creates or retrieves an Int64UpDownCounter instrument from the underlying
// OTel meter. UpDownCounters are suitable for metrics that can increase or decrease, such as active requests.
func (t *Telemetry) MeterInt64UpDownCounter(metric Metric) (otelmetric.Int64UpDownCounter, error) { //nolint:ireturn
	counter, err := t.meter.Int64UpDownCounter(
		metric.Name,
		otelmetric.WithDescription(metric.Description),
		otelmetric.WithUnit(metric.Unit),
	)
	if err != nil {
		return nil, err
	}

	return counter, nil
}

// TraceStart initiates a new OTel trace span with the specified name using the internal tracer.
// The caller is responsible for ending the returned span (typically via defer span.End()).
func (t *Telemetry) TraceStart(ctx context.Context, name string) (context.Context, oteltrace.Span) {
	return t.Tracer.Start(ctx, name)
}

// Shutdown shuts down the logger, meter, and tracer providers.
// All errors are joined and returned so the caller knows if any cleanup failed.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	if t.lp != nil {
		errs = append(errs, t.lp.Shutdown(ctx))
	}
	if t.mp != nil {
		errs = append(errs, t.mp.Shutdown(ctx))
	}
	if t.tp != nil {
		errs = append(errs, t.tp.Shutdown(ctx))
	}

	return errors.Join(errs...)
}
