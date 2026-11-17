package agentrun

import (
	"encoding/json"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

func TestAgentRunToChatRequestReplaysPreviousRunToolResult(t *testing.T) {
	prevRunID := uuid.New()

	prevQuestion := "How much did I spend?"
	preToolText := "I'll check that."
	finalText := "You spent $42."
	toolDisplay := "Query transactions"
	toolSummary := "Fetched spending total"

	req := sharedModel.AgentRunCreateRequest{
		Message: "What about this month?",
		PrevMessages: []sharedModel.ConversationMessage{
			{
				Role: sharedModel.RoleUser,
				Content: rawMessageParts(t, []sharedModel.MessagePart{
					{Type: sharedModel.MessageTypeText, Content: &prevQuestion},
				}),
			},
			{
				RunID: &prevRunID,
				Role:  sharedModel.RoleAssistant,
				Content: rawMessageParts(t, []sharedModel.MessagePart{
					{Type: sharedModel.MessageTypeText, Content: &preToolText},
					{
						Type:        sharedModel.MessageTypeToolCall,
						DisplayName: &toolDisplay,
						Summary:     &toolSummary,
					},
					{Type: sharedModel.MessageTypeText, Content: &finalText},
				}),
			},
		},
		PrevRuns: []sharedModel.AgentRun{
			{
				ID: prevRunID,
				Metadata: map[string]any{
					"toolCalls": []map[string]any{
						{
							"id":     "call_1",
							"name":   "execute_sql",
							"args":   map[string]any{"sql": "select 42"},
							"result": toolResultMap("call_1", "execute_sql", `{"total":42}`),
						},
					},
				},
			},
		},
	}

	chatReq := agentRunToChatRequest(req)

	if got, want := len(chatReq.Messages), 5; got != want {
		t.Fatalf("message count = %d, want %d: %#v", got, want, chatReq.Messages)
	}
	if got := chatReq.Messages[0].Content[0].Text; got != prevQuestion {
		t.Fatalf("previous user message = %q, want %q", got, prevQuestion)
	}

	assistantToolCall := chatReq.Messages[1]
	if assistantToolCall.Role != sharedModel.RoleAssistant {
		t.Fatalf("tool-call message role = %q, want assistant", assistantToolCall.Role)
	}
	if got := assistantToolCall.Content[0].Text; got != preToolText {
		t.Fatalf("assistant pre-tool text = %q, want %q", got, preToolText)
	}
	if got, want := len(assistantToolCall.ToolCalls), 1; got != want {
		t.Fatalf("tool call count = %d, want %d", got, want)
	}
	if got := assistantToolCall.ToolCalls[0].ID; got != "call_1" {
		t.Fatalf("tool call id = %q, want call_1", got)
	}

	toolMessage := chatReq.Messages[2]
	if toolMessage.Role != sharedModel.RoleTool || toolMessage.ToolResult == nil {
		t.Fatalf("third message should be tool result: %#v", toolMessage)
	}
	if got := toolMessage.ToolResult.ToolCallId; got != "call_1" {
		t.Fatalf("tool result id = %q, want call_1", got)
	}
	if got := toolMessage.ToolResult.Content[0].Text; got != `{"total":42}` {
		t.Fatalf("tool result text = %q, want result JSON", got)
	}

	if got := chatReq.Messages[3].Content[0].Text; got != finalText {
		t.Fatalf("assistant final text = %q, want %q", got, finalText)
	}
	if got := chatReq.Messages[4].Content[0].Text; got != req.Message {
		t.Fatalf("current user message = %q, want %q", got, req.Message)
	}
}

func TestAgentRunToChatRequestReplaysHiddenToolResultBeforeAssistantText(t *testing.T) {
	prevRunID := uuid.New()
	finalText := "I checked the schema."

	req := sharedModel.AgentRunCreateRequest{
		Message: "Now run the query.",
		PrevMessages: []sharedModel.ConversationMessage{
			{
				RunID: &prevRunID,
				Role:  sharedModel.RoleAssistant,
				Content: rawMessageParts(t, []sharedModel.MessagePart{
					{Type: sharedModel.MessageTypeText, Content: &finalText},
				}),
			},
		},
		PrevRuns: []sharedModel.AgentRun{
			{
				ID: prevRunID,
				Metadata: map[string]any{
					"toolCalls": []map[string]any{
						{
							"id":     "schema_1",
							"name":   "get_schema",
							"args":   map[string]any{"tables": []string{"transactions"}},
							"result": toolResultMap("schema_1", "get_schema", `{"tables":["transactions"]}`),
						},
					},
				},
			},
		},
	}

	chatReq := agentRunToChatRequest(req)

	if got, want := len(chatReq.Messages), 4; got != want {
		t.Fatalf("message count = %d, want %d: %#v", got, want, chatReq.Messages)
	}
	if chatReq.Messages[0].Role != sharedModel.RoleAssistant || len(chatReq.Messages[0].ToolCalls) != 1 {
		t.Fatalf("first message should replay hidden assistant tool call: %#v", chatReq.Messages[0])
	}
	if chatReq.Messages[1].Role != sharedModel.RoleTool || chatReq.Messages[1].ToolResult == nil {
		t.Fatalf("second message should replay hidden tool result: %#v", chatReq.Messages[1])
	}
	if got := chatReq.Messages[2].Content[0].Text; got != finalText {
		t.Fatalf("assistant text = %q, want %q", got, finalText)
	}
}

func rawMessageParts(t *testing.T, parts []sharedModel.MessagePart) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(parts)
	if err != nil {
		t.Fatalf("marshal message parts: %v", err)
	}
	return data
}

func toolResultMap(toolCallID, name, text string) map[string]any {
	return map[string]any{
		"ToolCallId": toolCallID,
		"Name":       name,
		"Content": []map[string]any{
			{
				"Type": "text",
				"Text": text,
			},
		},
		"IsError": false,
	}
}

func TestRunToolCallsPrefersToolInvocations(t *testing.T) {
	result, _ := json.Marshal(sharedModel.ToolResult{
		ToolCallId: "call_1",
		Name:       "execute_sql",
		Content:    []sharedModel.ContentBlock{{Type: "text", Text: `[{"total":42}]`}},
	})
	run := sharedModel.AgentRun{
		// An older copy in metadata must not win over the rows.
		Metadata: map[string]any{"toolCalls": []map[string]any{{"id": "old", "name": "old"}}},
		ToolInvocations: []sharedModel.ToolInvocation{{
			Sequence: 1, Turn: 1, ToolCallID: "call_1", ToolName: "execute_sql",
			Input: json.RawMessage(`{"query":"select 1"}`), Output: result,
		}},
	}

	calls := runToolCalls(run)
	if len(calls) != 1 || calls[0].ID != "call_1" || calls[0].Name != "execute_sql" {
		t.Fatalf("calls = %+v, want the tool_invocations row", calls)
	}
	if calls[0].Args["query"] != "select 1" {
		t.Fatalf("args = %v", calls[0].Args)
	}
	got := toolResultFromRaw(calls[0].Result, sharedModel.ToolCall{ID: "call_1", Name: "execute_sql"})
	if messageText(got.Content) != `[{"total":42}]` {
		t.Fatalf("replayed result = %q", messageText(got.Content))
	}
}

func TestRunToolCallsReplaysTruncatedResultAsText(t *testing.T) {
	run := sharedModel.AgentRun{ToolInvocations: []sharedModel.ToolInvocation{{
		Sequence: 1, Turn: 1, ToolCallID: "call_1", ToolName: "execute_sql", Truncated: true,
		Input:  json.RawMessage(`{}`),
		Output: json.RawMessage(`{"truncated":true,"bytes":90000,"preview":"{\"rows\":[1,2,"}`),
	}}}

	calls := runToolCalls(run)
	got := toolResultFromRaw(calls[0].Result, sharedModel.ToolCall{ID: "call_1", Name: "execute_sql"})
	text := messageText(got.Content)
	if text == "" || !json.Valid(calls[0].Result) {
		t.Fatalf("truncated result replayed as %q", text)
	}
	if want := `{"rows":[1,2,`; len(text) < len(want) || text[len(text)-len(want):] != want {
		t.Fatalf("replayed text %q should end with the preview", text)
	}
}

func TestRunToolCallsFallsBackToMetadata(t *testing.T) {
	run := sharedModel.AgentRun{Metadata: map[string]any{"toolCalls": []any{
		map[string]any{"id": "call_9", "name": "get_today", "args": map[string]any{}, "result": map[string]any{}},
	}}}
	calls := runToolCalls(run)
	if len(calls) != 1 || calls[0].ID != "call_9" {
		t.Fatalf("calls = %+v, want the metadata copy for a run from before tool_invocations", calls)
	}
}
