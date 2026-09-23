package llm

import (
	"context"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/telemetry"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const spanContentLimit = 20_000

func recordLLMContext(ctx context.Context, span trace.Span, req sharedModel.ChatRequest, conversationID *string) {
	if conversationID != nil && *conversationID != "" {
		span.SetAttributes(attribute.String("langfuse.session.id", *conversationID))
	}
	traceName := "agent.run"
	switch req.ChatType {
	case sharedModel.MemoryChatType:
		traceName = "memory.observe"
	case sharedModel.EmailPredictChatType:
		traceName = "email.predict"
	}
	span.SetAttributes(attribute.String("langfuse.trace.name", traceName))
	if runID := req.Metadata["runId"]; runID != "" {
		span.SetAttributes(attribute.String("langfuse.trace.metadata.run_id", runID))
	}
	if userID, err := utils.UserIDFromContext(ctx); err == nil {
		span.SetAttributes(attribute.String("langfuse.user.id", userID.String()))
	}
}

func recordUsage(span trace.Span, usage sharedModel.Usage) {
	if usage.Available {
		span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", usage.InputTokens),
			attribute.Int("gen_ai.usage.output_tokens", usage.OutputTokens),
			attribute.Int("gen_ai.usage.total_tokens", usage.TotalTokens),
		)
	}
}

// LLM is the provider-neutral interface all LLM clients must satisfy.
type LLM interface {
	Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error)
	Stream(ctx context.Context, req sharedModel.ChatRequest) <-chan sharedModel.StreamChunk
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

func (o *ObservedLLM) Chat(
	ctx context.Context,
	req sharedModel.ChatRequest,
	conversationID *string,
) (*sharedModel.ChatResponse, error) {
	ctx, span := o.telemetry.TraceStartWithScope(ctx, otelSDK.LLMScope, "chat "+req.Model)
	defer span.End()

	serializedMessages, marshalErr := telemetry.JSON(req.Messages, spanContentLimit)
	if marshalErr != nil {
		span.RecordError(marshalErr)
	}

	recordLLMContext(ctx, span, req, conversationID)

	span.SetAttributes(
		attribute.String("gen_ai.request.model", req.Model),
		attribute.Int("gen_ai.request.max_tokens", req.MaxTokens),
		attribute.Int("gen_ai.request.tool_count", len(req.Tools)),
		attribute.String("gen_ai.prompt", serializedMessages),
		attribute.String("langfuse.observation.input", serializedMessages),
	)

	res, err := o.client.Chat(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	serializedMessage, marshalErr := telemetry.JSON(res.Message.Content, spanContentLimit)
	if marshalErr != nil {
		span.RecordError(marshalErr)
	}

	span.SetAttributes(
		attribute.String("gen_ai.completion", serializedMessage),
		attribute.String("langfuse.observation.output", serializedMessage),
		attribute.String("gen_ai.response.model", res.Model),
		attribute.String("gen_ai.response.finish_reason", string(res.StopReason)),
		attribute.Int("gen_ai.response.tool_call_count", len(res.Message.ToolCalls)),
	)
	recordUsage(span, res.Usage)

	span.SetStatus(codes.Ok, "")

	return res, nil
}

func (o *ObservedLLM) Stream(
	ctx context.Context,
	req sharedModel.ChatRequest,
	conversationID *string,
) <-chan sharedModel.StreamChunk {
	ctx, span := o.telemetry.TraceStartWithScope(ctx, otelSDK.LLMScope, "chat "+req.Model)

	serializedMessages, marshalErr := telemetry.JSON(req.Messages, spanContentLimit)
	if marshalErr != nil {
		span.RecordError(marshalErr)
	}

	logger.Logger(ctx).
		Info("Chatting with agent", "chat_type", req.ChatType, "spanId", span.SpanContext().SpanID(), "model", req.Model)

	recordLLMContext(ctx, span, req, conversationID)
	span.SetAttributes(
		attribute.String("gen_ai.request.model", req.Model),
		attribute.Int("gen_ai.request.max_tokens", req.MaxTokens),
		attribute.Int("gen_ai.request.tool_count", len(req.Tools)),
		attribute.String("gen_ai.prompt", serializedMessages),
		attribute.String("langfuse.observation.input", serializedMessages),
	)

	upstream := o.client.Stream(ctx, req)
	out := make(chan sharedModel.StreamChunk)

	go func() {
		defer span.End()
		defer close(out)
		var output strings.Builder
		firstToken := false
		defer func() {
			if output.Len() > 0 {
				serializedOutput, err := telemetry.JSON(output.String(), spanContentLimit)
				if err != nil {
					span.RecordError(err)
				} else {
					span.SetAttributes(
						attribute.String("langfuse.observation.output", serializedOutput),
						attribute.String("gen_ai.completion", serializedOutput),
					)
				}
			}
		}()
		for chunk := range upstream {
			switch chunk.Type {
			case sharedModel.ChunkEventReasoning:
				// Provider-supplied summaries are forwarded to the UI, not included
				// in the final answer or the time-to-first-answer-token metric.
			case sharedModel.ChunkEventText:
				if chunk.Text != "" {
					if !firstToken {
						span.SetAttributes(attribute.String("langfuse.observation.completion_start_time", time.Now().UTC().Format(time.RFC3339Nano)))
						firstToken = true
					}
					if output.Len() < spanContentLimit {
						remaining := spanContentLimit - output.Len()
						if len(chunk.Text) > remaining {
							output.WriteString(chunk.Text[:remaining])
						} else {
							output.WriteString(chunk.Text)
						}
					}
				}
			case sharedModel.ChunkEventCompleted:
				recordUsage(span, chunk.Usage)
				span.SetStatus(codes.Ok, "")
			case sharedModel.ChunkEventError:
				span.SetAttributes(attribute.String("gen_ai.stream.error", chunk.Text))
				span.SetStatus(codes.Error, chunk.Text)
			}
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
