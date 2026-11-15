package otelSDK

import (
	"context"
	"encoding/base64"
	"os"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"

	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// newLangfuseExporter defines custom exporter for langfuse
// it skips configuration if no public or secret key is set
func newLangfuseExporter(ctx context.Context, cfg Config) (*otlptrace.Exporter, error) {
	if !cfg.LangfuseOtelEnabled || cfg.LangfusePublicKey == "" || cfg.LangfuseSecretKey == "" {
		return nil, nil
	}
	authString := base64.RawStdEncoding.EncodeToString([]byte(cfg.LangfusePublicKey + ":" + cfg.LangfuseSecretKey))

	exporter, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpointURL(cfg.LangfuseURL+"/api/public/otel/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization":                "Basic " + authString,
			"x-langfuse-ingestion-version": "4",
		}),
	)
	if err != nil {
		return nil, err
	}
	return exporter, nil
}

// newTracerProvider creates and configures a new OpenTelemetry TracerProvider.
// The exporter comes from the standard OTEL_TRACES_EXPORTER and
// OTEL_EXPORTER_OTLP_* environment variables (autoexport).
func newTracerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}

	// Use autoexport to detect the span exporter type and create it from the env
	spanExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error creating span exporter", err)
	}

	langfuseExporter, err := newLangfuseExporter(ctx, cfg)
	if err != nil {
		// Cleanup span exporter
		spanExporter.Shutdown(ctx)
		return nil, errs.Wrap(errs.CodeInternalError, "error while creating langfuse exporter", err)
	}

	opts = append(opts, sdktrace.WithBatcher(spanExporter))

	if langfuseExporter != nil {
		opts = append(opts, sdktrace.WithBatcher(langfuseAIExporter{next: langfuseExporter}))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)

	return tp, nil
}

// newMeterProvider creates and configures a new OpenTelemetry MeterProvider.
// It sets up a metric exporter (currently stdout) and a periodic reader to
// periodically flush metric data. The provider is registered globally.
func newMeterProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	opts := []sdkmetric.Option{sdkmetric.WithResource(res)}

	reader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error creating metric reader", err)
	}

	opts = append(opts, sdkmetric.WithReader(reader))
	meterProvider := sdkmetric.NewMeterProvider(opts...)
	otel.SetMeterProvider(meterProvider)
	return meterProvider, nil
}

// newLoggerProvider creates and configures a new OpenTelemetry LoggerProvider.
// It uses a BatchProcessor to efficiently buffer log entries before sending
// them to the configured exporter (currently stdout).
// Note: In production, this can be bridged with slog to route all application
// logs through the OTel pipeline.
func newLoggerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	opts := []sdklog.LoggerProviderOption{
		sdklog.WithResource(res),
	}

	exporter, err := autoexport.NewLogExporter(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error creating log exporter", err)
	}

	opts = append(opts, sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)))

	lp := sdklog.NewLoggerProvider(opts...)
	return lp, nil
}

// newResource creates an OpenTelemetry Resource that identifies the source
// of the telemetry data. It attaches standard attributes such as the service
// name, service version, hostname, and deployment environment to all generated
// spans, metrics, and logs. Langfuse uses deployment.environment to scope
// traces per environment (e.g. "production" vs "development").
func newResource(serviceName string, serviceVersion string, environment string) *resource.Resource {
	hostName, _ := os.Hostname()

	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.HostName(hostName),
		semconv.DeploymentEnvironment(environment),
	)
}

// newPropagator creates a composite propagator that enables distributed trace
// context propagation across service boundaries via HTTP headers.
// TraceContext handles the W3C `traceparent` header, Baggage handles `baggage`.
func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
