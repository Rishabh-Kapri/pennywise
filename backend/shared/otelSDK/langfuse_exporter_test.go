package otelSDK

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestLangfuseExporterMakesAgentTheRoot(t *testing.T) {
	sink := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(langfuseAIExporter{next: sink}))
	defer provider.Shutdown(context.Background())

	ctx, httpSpan := provider.Tracer("http").Start(context.Background(), "POST /api/agent/runs")
	ctx, agentSpan := provider.Tracer(AgentScope).Start(ctx, "agent.run")
	agentID := agentSpan.SpanContext().SpanID()
	agentSpan.SetAttributes(
		attribute.String("langfuse.observation.type", "agent"),
		attribute.String("langfuse.observation.input", `"question"`),
		attribute.String("langfuse.observation.output", `"answer"`),
	)
	_, llmSpan := provider.Tracer(LLMScope).Start(ctx, "chat lumo-lite")
	llmSpan.End()
	_, toolSpan := provider.Tracer(AgentScope).Start(ctx, "execute_tool lookup")
	toolSpan.SetAttributes(attribute.String("langfuse.observation.type", "tool"))
	toolSpan.End()
	agentSpan.End()
	httpSpan.End()

	spans := sink.GetSpans()
	if len(spans) != 3 {
		t.Fatalf("exported %d spans, want agent, LLM, and tool", len(spans))
	}
	for _, span := range spans {
		switch span.Name {
		case "agent.run":
			if span.Parent.IsValid() {
				t.Fatalf("agent parent %s should be absent in Langfuse", span.Parent.SpanID())
			}
			var input, output bool
			for _, attr := range span.Attributes {
				input = input || attr.Key == "langfuse.observation.input"
				output = output || attr.Key == "langfuse.observation.output"
			}
			if !input || !output {
				t.Fatal("agent input or output was dropped")
			}
		case "chat lumo-lite", "execute_tool lookup":
			if span.Parent.SpanID() != agentID {
				t.Fatalf("%s parent = %s, want agent %s", span.Name, span.Parent.SpanID(), agentID)
			}
		default:
			t.Fatalf("unexpected exported span %q", span.Name)
		}
	}
}
