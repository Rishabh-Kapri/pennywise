package memory

import (
	"context"
	"strings"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// approxTokenCounter stands in for tiktoken, which downloads its encoding file
// on first use and so cannot run in a unit test. Character count is a fine
// proxy: these tests care about which messages survive, not exact token maths.
func approxTokenCounter(messages []sharedModel.AgentMessage, _ int) (int, error) {
	total := 0
	for _, msg := range messages {
		for _, block := range msg.Content {
			total += len(block.Text)
		}
		if msg.ToolResult != nil {
			for _, block := range msg.ToolResult.Content {
				total += len(block.Text)
			}
		}
		total += len(msg.ToolCalls) * 16
	}
	return total, nil
}

func newTestMemory(messageTokens int) *memory {
	return &memory{messageTokens: messageTokens, countTokensFn: approxTokenCounter}
}

func textMessage(role sharedModel.Role, sequence int, text string) sharedModel.AgentMessage {
	return sharedModel.AgentMessage{
		Role:     role,
		Sequence: sequence,
		Content:  []sharedModel.ContentBlock{{Type: "text", Text: text}},
	}
}

func toolCallMessage(sequence int, callID string) sharedModel.AgentMessage {
	return sharedModel.AgentMessage{
		Role:      sharedModel.RoleAssistant,
		Sequence:  sequence,
		ToolCalls: []sharedModel.ToolCall{{ID: callID, Name: "execute_sql"}},
	}
}

func toolResultMessage(sequence int, callID, payload string) sharedModel.AgentMessage {
	return sharedModel.AgentMessage{
		Role:     sharedModel.RoleTool,
		Sequence: sequence,
		ToolResult: &sharedModel.ToolResult{
			ToolCallId: callID,
			Name:       "execute_sql",
			Content:    []sharedModel.ContentBlock{{Type: "text", Text: payload}},
		},
	}
}

// Every tool_use must keep a matching tool_result: an unanswered one is rejected
// by the provider, which is the failure this whole enforcement path exists to
// avoid causing.
func assertToolCallsAnswered(t *testing.T, messages []sharedModel.AgentMessage) {
	t.Helper()

	answered := map[string]bool{}
	for _, msg := range messages {
		if msg.ToolResult != nil {
			answered[msg.ToolResult.ToolCallId] = true
		}
	}
	for _, msg := range messages {
		for _, call := range msg.ToolCalls {
			if !answered[call.ID] {
				t.Fatalf("tool call %q has no matching tool result: %#v", call.ID, messages)
			}
		}
	}
}

func TestEnforceTokenBudgetLeavesSmallConversationAlone(t *testing.T) {
	m := newTestMemory(8000)

	messages := []sharedModel.AgentMessage{
		textMessage(sharedModel.RoleSystem, 0, "system"),
		textMessage(sharedModel.RoleUser, 1, "how much did I spend?"),
		textMessage(sharedModel.RoleAssistant, 1, "₹42"),
	}

	got := m.enforceTokenBudget(context.Background(), messages)

	if len(got) != len(messages) {
		t.Fatalf("message count = %d, want %d", len(got), len(messages))
	}
}

// Tool output is the dominant source of growth, so it must be shrunk before any
// conversational turn is dropped.
func TestEnforceTokenBudgetShrinksOldToolResultsFirst(t *testing.T) {
	m := newTestMemory(500)

	bigPayload := strings.Repeat("row of transaction json, ", 400)
	messages := []sharedModel.AgentMessage{
		textMessage(sharedModel.RoleSystem, 0, "system"),
		textMessage(sharedModel.RoleUser, 1, "first question"),
		toolCallMessage(1, "call_1"),
		toolResultMessage(1, "call_1", bigPayload),
		textMessage(sharedModel.RoleAssistant, 1, "first answer"),
		textMessage(sharedModel.RoleUser, 2, "second question"),
		textMessage(sharedModel.RoleAssistant, 2, "second answer"),
		textMessage(sharedModel.RoleUser, 3, "third question"),
		textMessage(sharedModel.RoleAssistant, 3, "third answer"),
		textMessage(sharedModel.RoleUser, 4, "latest question"),
	}

	got := m.enforceTokenBudget(context.Background(), messages)

	assertToolCallsAnswered(t, got)

	var found bool
	for _, msg := range got {
		if msg.ToolResult != nil && msg.ToolResult.Content[0].Text == toolResultPlaceholder {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the old tool result to be replaced with a placeholder: %#v", got)
	}

	// The user's own turns should survive when shrinking alone was enough.
	if got[len(got)-1].Content[0].Text != "latest question" {
		t.Fatalf("latest user message must be preserved, got %#v", got[len(got)-1])
	}
}

// When shrinking is not enough, dropping must remove the assistant tool-call
// message and its results together.
func TestEnforceTokenBudgetDropsToolGroupsTogether(t *testing.T) {
	m := newTestMemory(50)

	filler := strings.Repeat("padding text ", 200)
	messages := []sharedModel.AgentMessage{
		textMessage(sharedModel.RoleSystem, 0, "system"),
		textMessage(sharedModel.RoleUser, 1, filler),
		toolCallMessage(1, "call_1"),
		toolResultMessage(1, "call_1", filler),
		textMessage(sharedModel.RoleAssistant, 1, filler),
		textMessage(sharedModel.RoleUser, 2, filler),
		toolCallMessage(2, "call_2"),
		toolResultMessage(2, "call_2", filler),
		textMessage(sharedModel.RoleAssistant, 2, filler),
		textMessage(sharedModel.RoleUser, 3, "latest question"),
	}

	got := m.enforceTokenBudget(context.Background(), messages)

	assertToolCallsAnswered(t, got)

	if len(got) >= len(messages) {
		t.Fatalf("expected messages to be dropped, got %d of %d", len(got), len(messages))
	}
	if last := got[len(got)-1]; last.Role != sharedModel.RoleUser || last.Content[0].Text != "latest question" {
		t.Fatalf("latest user message must never be dropped, got %#v", last)
	}

	var systemCount int
	for _, msg := range got {
		if msg.Role == sharedModel.RoleSystem {
			systemCount++
		}
	}
	if systemCount < 2 {
		t.Errorf("expected an omission marker alongside the system prompt, got %d system messages", systemCount)
	}
}

// A conversation with nothing droppable must be returned unchanged rather than
// looping or stripping the final turn.
func TestEnforceTokenBudgetStopsWhenNothingIsDroppable(t *testing.T) {
	m := newTestMemory(1)

	messages := []sharedModel.AgentMessage{
		textMessage(sharedModel.RoleSystem, 0, strings.Repeat("system ", 500)),
		textMessage(sharedModel.RoleUser, 1, strings.Repeat("question ", 500)),
	}

	got := m.enforceTokenBudget(context.Background(), messages)

	if len(got) != 2 {
		t.Fatalf("message count = %d, want 2 (nothing safe to drop)", len(got))
	}
}
