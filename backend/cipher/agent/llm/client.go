package llm

import (
	"context"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const spanContentLimit = 20_000

// LLM is the provider-neutral interface all LLM clients must satisfy.
type LLM interface {
	Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error)
	Stream(ctx context.Context, req sharedModel.ChatRequest) <-chan sharedModel.StreamChunk
}

// LLMResolver resolves a provider + model name to a ready-to-use ObservedLLM.
// It returns the resolved model name (applying defaults when the caller passes "")
// so the caller can set req.Model correctly.
type LLMResolver interface {
	Resolve(provider, model string) (*ObservedLLM, string, error)
}

// ObservedLLM wraps any LLM with OpenTelemetry tracing.
// It holds the TelemetryProvider interface so a no-op provider can be
// substituted safely (avoids nil-dereference on zero-value Telemetry struct).
type ObservedLLM struct {
	client    LLM
	telemetry otelSDK.TelemetryProvider
}

func NewObservedLLM(client LLM, tel otelSDK.TelemetryProvider) *ObservedLLM {
	return &ObservedLLM{
		client:    client,
		telemetry: tel,
	}
}

func (o *ObservedLLM) Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error) {
	ctx, span := o.telemetry.TraceStart(ctx, "llm.chat")
	defer span.End()

	serializedMessages, marshalErr := utils.Marshal(req.Messages, spanContentLimit)
	if marshalErr != nil {
		span.RecordError(marshalErr)
	}

	span.SetAttributes(
		attribute.String("gen_ai.request.model", req.Model),
		attribute.Int("gen_ai.request.max_tokens", req.MaxTokens),
		attribute.Int("gen_ai.request.tool_count", len(req.Tools)),
		attribute.String("gen_ai.prompt", string(serializedMessages)),
	)

	res, err := o.client.Chat(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	serializedMessage, marshalErr := utils.Marshal(res.Message, spanContentLimit)
	if marshalErr != nil {
		span.RecordError(marshalErr)
	}

	span.SetAttributes(
		attribute.String("gen_ai.completion", string(serializedMessage)),
		attribute.String("gen_ai.response.model", res.Model),
		attribute.String("gen_ai.response.finish_reason", string(res.StopReason)),
		attribute.Int("gen_ai.usage.input_tokens", res.Usage.InputTokens),
		attribute.Int("gen_ai.usage.output_tokens", res.Usage.OutputTokens),
		attribute.Int("gen_ai.usage.total_tokens", res.Usage.TotalTokens),
		attribute.Int("gen_ai.response.tool_call_count", len(res.Message.ToolCalls)),
	)

	span.SetStatus(codes.Ok, "")

	return res, nil
}

func (o *ObservedLLM) Stream(ctx context.Context, req sharedModel.ChatRequest) <-chan sharedModel.StreamChunk {
	ctx, span := o.telemetry.TraceStart(ctx, "llm.stream")

	serializedMessages, marshalErr := utils.Marshal(req.Messages, spanContentLimit)
	if marshalErr != nil {
		span.RecordError(marshalErr)
	}
	span.SetAttributes(
		attribute.String("gen_ai.request.model", req.Model),
		attribute.Int("gen_ai.request.max_tokens", req.MaxTokens),
		attribute.Int("gen_ai.request.tool_count", len(req.Tools)),
		attribute.String("gen_ai.prompt", string(serializedMessages)),
	)

	upstream := o.client.Stream(ctx, req)
	out := make(chan sharedModel.StreamChunk)

	go func() {
		defer span.End()
		defer close(out)
		for chunk := range upstream {
			switch chunk.Type {
			case sharedModel.ChunkEventCompleted:
				span.SetAttributes(
					attribute.Int("gen_ai.usage.input_tokens", chunk.Usage.InputTokens),
					attribute.Int("gen_ai.usage.output_tokens", chunk.Usage.OutputTokens),
					attribute.Int("gen_ai.usage.total_tokens", chunk.Usage.TotalTokens),
				)
				span.SetStatus(codes.Ok, "")
			case sharedModel.ChunkEventError:
				span.SetAttributes(attribute.String("gen_ai.stream.error", chunk.Text))
				span.SetStatus(codes.Error, chunk.Text)
			}
			out <- chunk
		}
	}()

	return out
}
