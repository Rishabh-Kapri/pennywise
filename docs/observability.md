# Observability: OpenTelemetry, Logging, and Langfuse

This document covers the observability stack across Pennywise services — why it was designed the way it was, what is currently implemented, and the production upgrade path.

---

## 1. Why OpenTelemetry?

Pennywise runs multiple services (`go-gmail`, `cipher`, `go-pennywise-api`, `python-mlp`) that interact in a non-trivial flow. A transaction email triggers `go-gmail`, which calls `python-mlp` for a prediction, which calls `cipher`, which makes multiple calls to Ollama and OpenAI. Understanding what is happening — latency, failure rates, model performance — across all of this without a unified observability layer would require manually grepping logs across services.

The core problems that needed solving:

- **LLM latency is variable and opaque.** A `POST /predict` taking 15 seconds could be a slow Ollama cold start, a slow OpenAI response, or a slow pgvector query. Without tracing, it is impossible to know which.
- **Token usage and cost are invisible.** Every OpenAI call has a dollar cost. Without instrumenting the calls, there is no way to know which requests are expensive or why.
- **Structured logs across services are not correlated.** Each service logs independently. A correlation ID links logs within a single service, but not across service boundaries.
- **Metrics require polling.** Understanding request throughput or error rates requires querying logs. A metrics layer provides aggregated, time-series data without log parsing.

[OpenTelemetry (OTel)](https://opentelemetry.io/) was chosen because it is the CNCF-standard, vendor-neutral observability framework. It decouples _what_ gets instrumented (spans, metrics, logs) from _where_ data goes (Langfuse, Grafana, Loki). Swapping a backend requires only changing an exporter configuration — no instrumentation code changes.

---

## 2. The Three Pillars and How They Map to Pennywise

| Pillar      | What it answers                                               | Implemented backend                                   |
| ----------- | ------------------------------------------------------------- | ----------------------------------------------------- |
| **Traces**  | What path did this request take, how long did each step take? | `stdout` (dev) → Langfuse (prod, LLM traces)          |
| **Metrics** | How is the service performing right now?                      | `stdout` (dev) → Grafana/Prometheus (prod)            |
| **Logs**    | What happened at a specific moment?                           | `slog` → `stdout` (dev) → Loki via OTel bridge (prod) |

---

## 3. The `otelSDK` Shared Package

All OTel infrastructure lives in `backend/shared/otelSDK/`. This package is imported by each service that requires telemetry, currently only `cipher`.

### Package structure

```
backend/shared/otelSDK/
├── config.go       — Reads SERVICE_NAME, SERVICE_VERSION, env vars
├── providers.go    — Creates LoggerProvider, MeterProvider, TracerProvider
└── otel.go         — Telemetry struct, TelemetryProvider interface, constructor
```

### The `Telemetry` struct

```go
type Telemetry struct {
    lp     *log.LoggerProvider    // manages log export pipeline
    mp     *metric.MeterProvider  // manages metric export pipeline
    tp     *trace.TracerProvider  // manages trace export pipeline
    meter  otelmetric.Meter       // creates metric instruments
    tracer oteltrace.Tracer       // creates spans
    cfg    Config                 // service name, version
}
```

The `TelemetryProvider` interface abstracts the struct for testability and future implementation swaps.

### Why `Telemetry` does not hold a `slog.Logger`

An earlier version of this struct wrapped a `slog.Logger` and exposed `LogInfo`, `LogError`, etc. methods. This was removed because:

1. `Telemetry` is not request-scoped — it is a singleton initialized at startup. An `slog.Logger` held inside it cannot carry `correlation_id`, `budget_id`, or `user_id`, which are injected per-request via `context.Context`. The existing `logger.Logger(ctx)` pattern in `backend/shared/logger/` already handles this correctly.
2. It created confusion — callers could use either `t.LogInfo(...)` (context-free) or `logger.Logger(ctx).Info(...)` (context-enriched), with the former producing inferior, contextless logs.

Application logging and OTel telemetry are separate responsibilities. `slog` handles the former; OTel handles the latter.

---

## 4. Application Logging (`backend/shared/logger/`)

The `logger` package wraps Go's standard `log/slog` and is used for all application-level logging across every service.

```go
// Setup configures the global slog default for the given service.
// In local/dev: colorized tint output with debug level.
// In production: structured JSON output with info level.
func Setup(service string)

// Logger returns an slog.Logger enriched from context.
// Automatically stamps every log line with:
//   - correlation_id (from X-Correlation-ID header or generated)
//   - budget_id (if present in context)
//   - user_id (if present in context)
func Logger(ctx context.Context) *slog.Logger
```

The log level is derived from `RAILWAY_ENVIRONMENT_NAME`. `local` and `dev` emit `DEBUG`; anything else emits `INFO`.

### Usage pattern in services

```go
log := logger.Logger(ctx)
log.Info("prediction complete", "source", result.Source, "payee", result.Payee)
// → {"time":"...","level":"INFO","msg":"prediction complete","service":"cipher",
//    "correlation_id":"abc123","budget_id":"uuid","source":"VECTOR","payee":"Swiggy"}
```

---

## 5. OTel Providers and Exporters

All three providers are initialized in `providers.go` and currently use `stdout` exporters — the simplest possible backend that prints to the console. This is intentional for development.

### Trace provider

```go
traceExporter, _ := stdouttrace.New()
traceProvider := trace.NewTracerProvider(
    trace.WithBatcher(traceExporter, trace.WithBatchTimeout(time.Second)),
    trace.WithResource(res),
)
```

Spans are batched and flushed every second. The `resource` attached to every span carries `service.name`, `service.version`, and `host.name`.

### Meter provider

```go
metricExporter, _ := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
meterProvider := metric.NewMeterProvider(
    metric.WithReader(metric.NewPeriodicReader(metricExporter)),
    metric.WithResource(res),
)
```

The `PeriodicReader` wakes every 60 seconds and flushes all accumulated metric data points as a JSON blob to stdout. The current metrics are `request_duration_millis` (histogram) and `requests_inflight` (up-down counter), both populated by Gin middleware.

### Logger provider

```go
logExporter, _ := stdoutlog.New(stdoutlog.WithPrettyPrint())
batchProcessor := log.NewBatchProcessor(logExporter)
loggerProvider := log.NewLoggerProvider(
    log.WithProcessor(batchProcessor),
    log.WithResource(res),
)
```

The `LoggerProvider` is initialized but not yet bridged to `slog`. In production, this provider is where the `slog`-to-OTel bridge connects (see §7).

---

## 6. Cipher Gin Middleware

`cipher/cmd/api/main.go` registers `otelgin.Middleware` (from `go.opentelemetry.io/contrib/instrumentation/...`), which creates a trace span for each HTTP request and propagates W3C TraceContext headers. Request logging is `RequestLogger` from `backend/shared/middleware`.

The package once had `tel.LogRequest()`, `tel.MeterRequestDuration()` and `tel.MeterRequestsInFlight()` middlewares. No service registered them, so they were removed along with the `Metric` definitions; the git history has them if request metrics are wanted again.

---

The current `Telemetry` struct and `TelemetryProvider` interface wrap OTel in a custom abstraction. In hindsight, this is unnecessary and arguably harmful. The package was kept as-is for learning purposes, but the problems are worth documenting.

**OTel is already the abstraction.** Its entire purpose is vendor neutrality. Wrapping it "in case we swap OTel" is wrapping an abstraction in another abstraction — there is nothing to swap to.

**The abstraction leaks in three places:**

1. `TraceStartWithScope` returns `oteltrace.Span`. Callers must import `go.opentelemetry.io/otel/trace` to call `span.End()` and `span.SetAttributes(...)`. The wrapper provides no isolation.
2. (The metric instrument helpers that returned OTel metric types have been removed; they had no callers.)
3. `providers.go` sets global OTel state via `otel.SetMeterProvider` and `otel.SetTracerProvider`. Any third-party OTel middleware uses these globals, bypassing the wrapper entirely.

If you write an abstraction on top of a library but callers still need to import the underlying library, the abstraction is not providing value.

**What the package should be instead:** Helper functions that reduce init boilerplate and return OTel-native types. No struct, no interface.

```go
// Setup all providers, return a shutdown func. Consumers use otel.Tracer("cipher") directly.
func SetupProviders(ctx context.Context, cfg Config) (shutdown func(context.Context), err error)

// Standalone middleware that creates instruments from the global meter.
func MeterRequestDuration(serviceName string) gin.HandlerFunc
func MeterRequestsInFlight(serviceName string) gin.HandlerFunc
```

The valuable parts — provider wiring, metric definitions, middleware helpers, shutdown orchestration — remain. The unnecessary parts — the struct, the interface, the method receivers — go away.
