package llm

import (
	"context"
	"encoding/json"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type observationTestTelemetry struct{ provider *sdktrace.TracerProvider }

func (t observationTestTelemetry) GetServiceName() string { return "cipher-test" }
func (t observationTestTelemetry) MeterInt64Histogram(otelSDK.Metric) (metric.Int64Histogram, error) {
	return nil, nil
}
func (t observationTestTelemetry) MeterInt64UpDownCounter(otelSDK.Metric) (metric.Int64UpDownCounter, error) {
	return nil, nil
}
func (t observationTestTelemetry) TraceStart(ctx context.Context, name string) (context.Context, trace.Span) {
	return t.provider.Tracer("test").Start(ctx, name)
}
func (t observationTestTelemetry) TraceStartWithScope(ctx context.Context, scope, name string) (context.Context, trace.Span) {
	return t.provider.Tracer(scope).Start(ctx, name)
}
func (observationTestTelemetry) LogRequest() gin.HandlerFunc            { return nil }
func (observationTestTelemetry) MeterRequestDuration() gin.HandlerFunc  { return nil }
func (observationTestTelemetry) MeterRequestsInFlight() gin.HandlerFunc { return nil }
func (observationTestTelemetry) Shutdown(context.Context) error         { return nil }

type observationTestLLM struct{ chunks []sharedModel.StreamChunk }

func (observationTestLLM) Chat(context.Context, sharedModel.ChatRequest) (*sharedModel.ChatResponse, error) {
	return nil, nil
}
func (l observationTestLLM) Stream(context.Context, sharedModel.ChatRequest) <-chan sharedModel.StreamChunk {
	out := make(chan sharedModel.StreamChunk, len(l.chunks))
	for _, chunk := range l.chunks {
		out <- chunk
	}
	close(out)
	return out
}

func TestObservedStreamRecordsReasoningAndToolOutput(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(context.Background())
	client := NewObservedLLM(observationTestLLM{chunks: []sharedModel.StreamChunk{
		{Type: sharedModel.ChunkEventReasoning, Text: "Check the budget."},
		{Type: sharedModel.ChunkEventToolCallStart, OutputIndex: 0, ToolCallID: "call_1", ToolName: "get_budget"},
		{Type: sharedModel.ChunkEventToolCallDelta, OutputIndex: 0, ToolArgsDelta: `{"month":"September"}`},
		{Type: sharedModel.ChunkEventCompleted, Model: "lumo-lite"},
	}}, observationTestTelemetry{provider})
	req := sharedModel.ChatRequest{Model: "lumo-lite", Messages: []sharedModel.AgentMessage{{
		Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "Show my budget"}},
	}}}
	for range client.Stream(context.Background(), req, nil) {
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want one", len(spans))
	}
	attrs := make(map[attribute.Key]string)
	for _, attr := range spans[0].Attributes {
		attrs[attr.Key] = attr.Value.AsString()
	}
	if attrs["langfuse.observation.type"] != "generation" || attrs["langfuse.observation.metadata.reasoning"] != "Check the budget." {
		t.Fatalf("generation type or reasoning missing: %v", attrs)
	}
	if attrs["langfuse.observation.model.name"] != "lumo-lite" {
		t.Fatalf("actual model = %q", attrs["langfuse.observation.model.name"])
	}
	var input []map[string]any
	if err := json.Unmarshal([]byte(attrs["langfuse.observation.input"]), &input); err != nil || len(input) != 1 || input[0]["content"] != "Show my budget" {
		t.Fatalf("input = %q, error = %v", attrs["langfuse.observation.input"], err)
	}
	var output map[string]any
	err := json.Unmarshal([]byte(attrs["langfuse.observation.output"]), &output)
	calls, ok := output["tool_calls"].([]any)
	if err != nil || output["role"] != "assistant" || !ok || len(calls) != 1 {
		t.Fatalf("output = %q, error = %v", attrs["langfuse.observation.output"], err)
	}
}
