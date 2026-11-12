package otelSDK

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
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
	TraceStartWithScope(ctx context.Context, scope, name string) (context.Context, oteltrace.Span)
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
	logs := logger.Logger(ctx)
	logs.Info("telemetry init", "cfg", cfg.redacted())
	if cfg.OtelSdkDisabled {
		logs.Warn("otel sdk disabled")
		return &Telemetry{
			meter:  metricnoop.NewMeterProvider().Meter(cfg.ServiceName),
			Tracer: oteltrace.NewNoopTracerProvider().Tracer(cfg.ServiceName),
			cfg:    cfg,
		}, nil
	}
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
		logs.Error("otel error", "error", err)
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

	otelLogger := otelslog.NewLogger(
		cfg.ServiceName,
		otelslog.WithLoggerProvider(lp),
	)
	defaultLogger := slog.Default()
	slog.SetDefault(slog.New(
		slog.NewMultiHandler(
			otelLogger.Handler(),
			defaultLogger.Handler(),
		),
	))

	return &Telemetry{
		lp:     lp,
		mp:     mp,
		tp:     tp,
		meter:  meter,
		Tracer: tracer,
		cfg:    cfg,
	}, nil
}

// TraceStartWithScope records a span under an explicit instrumentation scope.
func (t *Telemetry) TraceStartWithScope(ctx context.Context, scope, name string) (context.Context, oteltrace.Span) {
	if t.tp == nil {
		return t.Tracer.Start(ctx, name)
	}
	return t.tp.Tracer(scope).Start(ctx, name)
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
