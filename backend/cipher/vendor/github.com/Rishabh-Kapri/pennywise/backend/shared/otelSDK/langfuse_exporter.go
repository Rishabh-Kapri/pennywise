package otelSDK

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// langfuseAIExporter limits only the Langfuse destination to AI spans.
// The primary exporter still receives the complete trace.
type langfuseAIExporter struct {
	next sdktrace.SpanExporter
}

func (e langfuseAIExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	aiSpans := make([]sdktrace.ReadOnlySpan, 0, len(spans))
	for _, span := range spans {
		switch span.InstrumentationScope().Name {
		case AgentScope, LLMScope, EmbeddingScope:
			aiSpans = append(aiSpans, span)
		}
	}
	if len(aiSpans) == 0 {
		return nil
	}
	return e.next.ExportSpans(ctx, aiSpans)
}

func (e langfuseAIExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}
