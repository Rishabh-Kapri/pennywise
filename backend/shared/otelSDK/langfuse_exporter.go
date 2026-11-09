package otelSDK

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Langfuse receives only AI spans. Make the agent entry point the root there,
// since its HTTP parent is exported only to the primary observability backend.
type langfuseRootSpan struct{ sdktrace.ReadOnlySpan }

func (langfuseRootSpan) Parent() trace.SpanContext { return trace.SpanContext{} }

func isAgentEntrySpan(span sdktrace.ReadOnlySpan) bool {
	if span.InstrumentationScope().Name != AgentScope {
		return false
	}
	for _, attr := range span.Attributes() {
		if attr.Key == attribute.Key("langfuse.observation.type") && attr.Value.AsString() == "agent" {
			return true
		}
	}
	return false
}

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
			if isAgentEntrySpan(span) {
				span = langfuseRootSpan{ReadOnlySpan: span}
			}
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
