package otelSDK

// Metric defines the configuration and metadata for an OpenTelemetry instrument.
// It centralizes the name, unit, and description so that middleware and handlers
// can instantiate metrics uniformly.
type Metric struct {
	Name        string
	Unit        string
	Description string
}

// MetricRequestDurationMillis defines a histogram metric used to record the
// latency (in milliseconds) of HTTP requests processed by the server.
var MetricRequestDurationMillis = Metric{
	Name:        "request_duration_millis",
	Unit:        "ms",
	Description: "Measures the latency of HTTP requests processed by the server, in milliseconds.",
}

// MetricRequestsInFlight defines an up-down counter metric used to track the
// real-time number of concurrent HTTP requests actively being processed.
var MetricRequestsInFlight = Metric{
	Name:        "requests_inflight",
	Unit:        "{count}",
	Description: "Measures the number of requests currently being processed by the server.",
}
