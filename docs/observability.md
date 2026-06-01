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
├── metrics.go      — Metric definitions (name, unit, description)
├── middlewares.go  — Gin middleware for request duration and inflight count
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

Four telemetry-related Gin middlewares are registered in `cipher/cmd/api/main.go`:

### `otelgin.Middleware` (from `go.opentelemetry.io/contrib/instrumentation/...`)

Automatically creates a trace span for each HTTP request, propagating W3C TraceContext headers. This integrates Gin request handling into the OTel trace tree.

### `tel.LogRequest()`

Logs each request (method, path, status, latency) at the `INFO` level via the shared `slog` logger, with correlation ID from context.

### `tel.MeterRequestDuration()`

Wraps every request with a timer. After `c.Next()` returns, it records the elapsed milliseconds into a histogram instrument. The histogram uses the default OTel bucket boundaries (`0, 5, 10, 25, 50, ... 10000 ms`).

> Note: Cipher makes calls to Ollama which can take 5–20 seconds. The default histogram boundaries max out at 10,000 ms. This should be overridden with service-specific boundaries such as `[0, 100, 500, 1000, 2500, 5000, 10000, 20000, 60000]` before hooking up a production metrics backend.

### `tel.MeterRequestsInFlight()`

Uses an `Int64UpDownCounter` that increments before `c.Next()` and decrements after. This gives a real-time concurrency gauge. At idle, the value is `0`. During a slow Ollama call, it reflects how many requests are being served simultaneously.

`MeterRequestDuration` and `MeterRequestsInFlight` attach HTTP semantic convention attributes (`http.method`, `net.host.name`, `user_agent.original`, etc.) via `httpconv.ServerRequest(...)`. These are standardized attribute names recognized by any OTel-compatible backend.

---

## 7. Production Upgrade Path

### 7.1 Swapping to OTLP exporters (metrics + traces)

In production, the `stdout*` exporters in `providers.go` are replaced with OTLP HTTP exporters pointing at a Grafana Alloy collector or a Prometheus-compatible endpoint. No instrumentation code changes — only the exporter initialization.

```go
// Replace stdoutmetric with:
import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"

exporter, _ := otlpmetrichttp.New(ctx,
    otlpmetrichttp.WithEndpointURL(cfg.OTLPEndpoint),
)
```

The `cfg.Env` field (read from `RAILWAY_ENVIRONMENT_NAME`) gates which exporter is created:

```go
if cfg.Env == "local" {
    // stdout exporter
} else {
    // OTLP exporter → Grafana Alloy
}
```

### 7.2 The `slog`-to-OTel bridge (logs to Loki)

Today, `slog` writes to `stdout`. In production on Railway, logs are collected from stdout by the platform's log drain. This is sufficient.

If a centralized log aggregator like Grafana Loki is added, logs can flow via the OTel Collector instead of stdout by installing the `slog` bridge:

```go
import "go.opentelemetry.io/contrib/bridges/otelslog"

// The bridge is an slog.Handler implementation.
// Instead of writing to os.Stdout, it converts each slog.Record
// into an OTel LogRecord and sends it to the LoggerProvider.
otelHandler := otelslog.NewHandler("cipher",
    otelslog.WithLoggerProvider(loggerProvider),
)

// In logger.Setup(), swap the handler based on environment:
if env == "local" {
    handler = tint.NewHandler(os.Stdout, ...)   // colorized stdout, unchanged
} else {
    handler = otelHandler                         // → OTel → Collector → Loki
}
```

No calling code changes. `logger.Logger(ctx).Info(...)` continues to work identically. The `LoggerProvider` already exists in `Telemetry`; wiring the bridge is a one-line change in `logger.Setup()`.

This means the complete production stack routes through a single OTel Collector endpoint:

```
cipher
├── slog (via bridge) ─────────► OTel Collector ──► Grafana Loki (logs)
├── MeterProvider (OTLP) ──────► OTel Collector ──► Grafana Mimir (metrics)
└── TracerProvider (OTLP) ─────► Langfuse (LLM traces)
                                 └── Grafana Tempo (distributed traces)
```

### 7.3 Langfuse integration (LLM traces)

Langfuse is an LLM observability platform that acts as an OTLP trace backend. It maps OTel spans carrying [GenAI semantic convention](https://opentelemetry.io/docs/specs/semconv/gen-ai/) attributes to its own data model, providing per-request visibility into model calls, token usage, cost, latency, and prompt/completion content.

**Why Langfuse for cipher specifically:** Cipher's `Predict` pipeline makes multiple sequential LLM calls — Phase 1 extraction via Ollama/Gemma, Phase 3 embedding via bge-m3, and Phase 4 classification via OpenAI. Each call has variable latency and cost. Without tracing, debugging a slow or incorrect prediction requires log-grepping. With Langfuse, the full trace tree is visible in a single UI.

#### Setup

Langfuse authenticates via Basic Auth. The base64-encoded `publicKey:secretKey` is passed as a request header to its OTLP endpoint.

Add to `cipher/.env`:

```env
# For self-hosted Langfuse, use your own domain or localhost
LANGFUSE_ENDPOINT=http://localhost:3000/api/public/otel
LANGFUSE_PUBLIC_KEY=pk-lf-...
LANGFUSE_SECRET_KEY=sk-lf-...
```

In `providers.go`, add a second trace exporter alongside `stdouttrace`:

```go
import (
    "encoding/base64"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

func newLangfuseExporter(ctx context.Context, cfg Config) (trace.SpanExporter, error) {
    if cfg.LangfusePublicKey == "" {
        return nil, nil
    }
    auth := base64.StdEncoding.EncodeToString(
        []byte(cfg.LangfusePublicKey + ":" + cfg.LangfuseSecretKey),
    )
    return otlptracehttp.New(ctx,
        otlptracehttp.WithEndpointURL(cfg.LangfuseEndpoint),
        otlptracehttp.WithHeaders(map[string]string{
            "Authorization":                "Basic " + auth,
            "x-langfuse-ingestion-version": "4",
        }),
    )
}
```

Both exporters attach to the same `TracerProvider` via `trace.WithBatcher(...)`. No code changes are needed in `predictionService` or anywhere else that creates spans — OTel routes each span to all registered exporters automatically.

#### Instrumenting LLM calls

Spans are created in `OllamaClient` and `doOpenAI` wrapping each network call. The GenAI semantic conventions tell Langfuse how to render them:

```go
// In doOpenAI / doLocalGenerate / Embed:
ctx, span := c.tracer.Start(ctx, "chat "+model,
    oteltrace.WithSpanKind(oteltrace.SpanKindClient),
)
defer span.End()

span.SetAttributes(
    attribute.String("gen_ai.system", "openai"),       // provider
    attribute.String("gen_ai.request.model", model),   // model name
    attribute.String("gen_ai.prompt", prompt),          // input → Langfuse "Input"
    attribute.String("gen_ai.completion", response),    // output → Langfuse "Output"
    // For OpenAI responses (token counts are in the response body):
    attribute.Int("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
    attribute.Int("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
)
```

Because `context.Context` propagates the active span, child spans created inside `Predict` (embed → generate → classify) automatically form a tree in Langfuse without any manual parent-child linking.

#### What Langfuse shows per prediction request

```
Trace: cipher-predict
├── Generation: "embed bge-m3"       (Phase 3: vector embedding)
│   ├── System: ollama
│   ├── Input: "debit SWIGGY"
│   └── Duration: 0.4s
│
├── Generation: "chat gemma4"        (Phase 1: email extraction)
│   ├── System: ollama
│   ├── Input: extraction prompt + raw email text
│   ├── Output: {"merchant": "SWIGGY", "amount": 500.0, ...}
│   └── Duration: 3.2s
│
└── Generation: "chat gpt-4.1"      (Phase 4: LLM fallback)
    ├── System: openai
    ├── Input: classification prompt
    ├── Output: {"merchantName": "Swiggy", "suggestedTag": "🍽️ Dining Out"...}
    ├── Input tokens: 850  → cost calculated automatically
    ├── Output tokens: 45
    └── Duration: 1.8s
```

---

## 8. `Shutdown` and Graceful Flush

All three providers buffer data before exporting. The `Shutdown(ctx)` method on `Telemetry` is deferred in each service's `main()`:

```go
tel, err := otelSDK.NewTelemetry(ctx, *otelConfig)
defer tel.Shutdown(ctx)
```

Without this, any buffered spans or metrics accumulated since the last flush would be silently dropped on process exit. This is especially important for traces — a slow batch flush (e.g., Langfuse network call) needs time to complete before the process terminates.

---

## 9. Design Note: The `Telemetry` Wrapper Is an Over-Abstraction

The current `Telemetry` struct and `TelemetryProvider` interface wrap OTel in a custom abstraction. In hindsight, this is unnecessary and arguably harmful. The package was kept as-is for learning purposes, but the problems are worth documenting.

**OTel is already the abstraction.** Its entire purpose is vendor neutrality. Wrapping it "in case we swap OTel" is wrapping an abstraction in another abstraction — there is nothing to swap to.

**The abstraction leaks in three places:**

1. `TraceStart` returns `oteltrace.Span`. Callers must import `go.opentelemetry.io/otel/trace` to call `span.End()` and `span.SetAttributes(...)`. The wrapper provides no isolation.
2. `MeterInt64Histogram` and `MeterInt64UpDownCounter` return OTel metric types. Once again, the caller depends on OTel directly.
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
