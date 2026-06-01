package agent

import (
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func setSpanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func setSpanJSON(span trace.Span, key string, value any) {
	serialized, err := utils.Marshal(value, agentSpanContentLimit)
	if err != nil {
		span.RecordError(err)
		return
	}

	span.SetAttributes(attribute.String(key, string(serialized)))
}

func recordToolRequest(span trace.Span, toolCall sharedModel.ToolCall) {
	span.SetAttributes(
		attribute.String("tool.name", toolCall.Name),
		attribute.String("tool.call_id", toolCall.ID),
	)
	setSpanJSON(span, "tool.arguments", toolCall.Arguments)
}

func recordToolResult(span trace.Span, toolResult *sharedModel.ToolResult) {
	setSpanJSON(span, "tool.result", toolResult)
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
	setSpanJSON(span, "agent.input", req.Messages)
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
	setSpanJSON(span, "agent.output", res.Message)
	span.SetAttributes(
		attribute.Int("agent.final_turn_count", turnCount),
		attribute.Int("agent.final_tool_call_count", totalToolCalls),
	)
	span.SetStatus(codes.Ok, "")
}
