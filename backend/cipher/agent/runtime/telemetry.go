package agent

import (
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/telemetry"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/tools"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func setSpanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func setSpanJSON(span trace.Span, key string, value any) {
	serialized, err := telemetry.JSON(value, agentSpanContentLimit)
	if err != nil {
		span.RecordError(err)
		return
	}

	span.SetAttributes(attribute.String(key, serialized))
}

func recordToolRequest(span trace.Span, toolCall sharedModel.ToolCall, tool tools.Tool) {
	span.SetAttributes(
		attribute.String("langfuse.observation.type", "tool"),
		attribute.String("gen_ai.tool.name", toolCall.Name),
		attribute.String("gen_ai.tool.type", "function"),
		attribute.String("gen_ai.tool.description", tool.Definition().Description),
		attribute.String("tool.call_id", toolCall.ID),
	)
	setSpanJSON(span, "tool.arguments", toolCall.Arguments)
	setSpanJSON(span, "langfuse.observation.input", toolCall.Arguments)
}

func recordToolResult(span trace.Span, toolResult *sharedModel.ToolResult) {
	setSpanJSON(span, "tool.result", toolResult)
	setSpanJSON(span, "langfuse.observation.output", toolResult)
	span.SetStatus(codes.Ok, "")
}

func recordAgentRunStart(span trace.Span, req sharedModel.ChatRequest, maxTurns int, maxToolCalls int) {
	span.SetAttributes(
		attribute.String("agent.model", req.Model),
		attribute.Int("agent.initial_message_count", len(req.Messages)),
		attribute.Int("agent.tool_count", len(req.Tools)),
		attribute.Int("agent.max_turns", maxTurns),
		attribute.Int("agent.max_tool_calls", maxToolCalls),
	)
	input := any(req.Messages)
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == sharedModel.RoleUser {
			input = req.Messages[i].Content
			break
		}
	}
	setSpanJSON(span, "agent.input", input)
	setSpanJSON(span, "langfuse.observation.input", input)
}

func recordAgentTurn(span trace.Span, turnCount int, messageCount int, totalToolCalls int) {
	span.SetAttributes(
		attribute.Int("agent.turn_count", turnCount),
		attribute.Int("agent.total_tool_calls", totalToolCalls),
	)
	span.AddEvent("agent.turn.started", trace.WithAttributes(
		attribute.Int("agent.turn", turnCount),
		attribute.Int("agent.message_count", messageCount),
	))
}

func recordAgentStopReason(span trace.Span, stopReason sharedModel.StopReason) {
	span.SetAttributes(attribute.String("agent.stop_reason", string(stopReason)))
}

func recordToolCallsRequested(span trace.Span, turnCount int, toolCallCount int) {
	span.AddEvent("agent.tool_calls.requested", trace.WithAttributes(
		attribute.Int("agent.turn", turnCount),
		attribute.Int("tool.call_count", toolCallCount),
	))
}

func recordTotalToolCalls(span trace.Span, totalToolCalls int) {
	span.SetAttributes(attribute.Int("agent.total_tool_calls", totalToolCalls))
}

func recordAgentSuccess(span trace.Span, res *sharedModel.ChatResponse, turnCount int, totalToolCalls int) {
	setSpanJSON(span, "agent.output", res.Message.Content)
	setSpanJSON(span, "langfuse.observation.output", res.Message.Content)
	span.SetAttributes(
		attribute.Int("agent.final_turn_count", turnCount),
		attribute.Int("agent.final_tool_call_count", totalToolCalls),
	)
	span.SetStatus(codes.Ok, "")
}
