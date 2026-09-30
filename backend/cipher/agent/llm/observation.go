package llm

import (
	"sort"
	"strings"
	"unicode/utf8"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func contentText(blocks []sharedModel.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func observationToolCalls(calls []sharedModel.ToolCall) []map[string]any {
	out := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		out = append(out, map[string]any{
			"id":   call.ID,
			"type": "function",
			"function": map[string]string{
				"name":      call.Name,
				"arguments": string(call.Arguments),
			},
		})
	}
	return out
}

func observationInput(messages []sharedModel.AgentMessage) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		if msg.ToolResult != nil {
			out = append(out, map[string]any{
				"role": "tool", "tool_call_id": msg.ToolResult.ToolCallId,
				"content": contentText(msg.ToolResult.Content),
			})
			continue
		}
		item := map[string]any{"role": string(msg.Role), "content": contentText(msg.Content)}
		if len(msg.ToolCalls) > 0 {
			item["tool_calls"] = observationToolCalls(msg.ToolCalls)
		}
		out = append(out, item)
	}
	return out
}

func observationOutput(text string, calls []sharedModel.ToolCall) any {
	if len(calls) == 0 {
		return text
	}
	return map[string]any{
		"role": "assistant", "content": text,
		"tool_calls": observationToolCalls(calls),
	}
}

func boundedText(value string) string {
	if len(value) <= spanContentLimit {
		return value
	}
	value = value[:spanContentLimit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func writeBounded(builder *strings.Builder, value string) {
	remaining := spanContentLimit - builder.Len()
	if remaining <= 0 {
		return
	}
	if len(value) > remaining {
		value = value[:remaining]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	builder.WriteString(value)
}

type streamedToolCall struct {
	id   string
	name string
	args strings.Builder
}

func orderedStreamToolCalls(tools map[int]*streamedToolCall) []sharedModel.ToolCall {
	indexes := make([]int, 0, len(tools))
	for index := range tools {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	calls := make([]sharedModel.ToolCall, 0, len(indexes))
	for _, index := range indexes {
		tool := tools[index]
		calls = append(calls, sharedModel.ToolCall{
			ID: tool.id, Name: tool.name, Arguments: []byte(tool.args.String()),
		})
	}
	return calls
}
