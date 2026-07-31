package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/tools"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// scriptedLLM returns a fixed sequence of responses, recording what it was sent
// so the message history the agent built can be inspected.
type scriptedLLM struct {
	responses []sharedModel.ChatResponse
	calls     int
	requests  []sharedModel.ChatRequest
}

func (s *scriptedLLM) Chat(_ context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error) {
	s.requests = append(s.requests, req)
	if s.calls >= len(s.responses) {
		return nil, errors.New("scriptedLLM: more calls than scripted responses")
	}
	res := s.responses[s.calls]
	s.calls++
	return &res, nil
}

func (s *scriptedLLM) Stream(context.Context, sharedModel.ChatRequest) <-chan sharedModel.StreamChunk {
	ch := make(chan sharedModel.StreamChunk)
	close(ch)
	return ch
}

// LLMResolver returns the concrete *llm.ObservedLLM, so the double is wrapped
// rather than substituted for it.
type scriptedResolver struct{ client *llm.ObservedLLM }

func (r scriptedResolver) Resolve(_ string, model string) (*llm.ObservedLLM, string, error) {
	if model == "" {
		model = "test-model"
	}
	return r.client, model, nil
}

// failingTool always errors, standing in for a bad SQL query.
type failingTool struct{ name string }

func (f failingTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{Name: f.name}
}

func (f failingTool) Execute(context.Context, sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	return nil, errors.New("relation \"transaction\" does not exist")
}

func (f failingTool) GetNormalizedName(bool) string { return "Queried data" }

func (f failingTool) Normalize(sharedModel.ToolCall, json.RawMessage) (*sharedModel.ToolResultNormalized, error) {
	return nil, errors.New("cannot normalize a failed result")
}

func toolUseResponse(callID, toolName string) sharedModel.ChatResponse {
	return sharedModel.ChatResponse{
		Message: sharedModel.AgentMessage{
			Role:      sharedModel.RoleAssistant,
			ToolCalls: []sharedModel.ToolCall{{ID: callID, Name: toolName, Arguments: json.RawMessage(`{}`)}},
		},
		StopReason: sharedModel.StopReasonToolUse,
	}
}

func endTurnResponse(text string) sharedModel.ChatResponse {
	return sharedModel.ChatResponse{
		Message: sharedModel.AgentMessage{
			Role:    sharedModel.RoleAssistant,
			Content: []sharedModel.ContentBlock{{Type: "text", Text: text}},
		},
		StopReason: sharedModel.StopReasonEndTurn,
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx := utils.WithBudgetID(context.Background(), uuid.New())
	return utils.WithUserID(ctx, uuid.New())
}

func newTestAgent(t *testing.T, client llm.LLM, registry *tools.ToolRegistry, opts ...AgentOption) *Agent {
	t.Helper()

	tel, err := otelSDK.NewTelemetry(context.Background(), otelSDK.Config{
		ServiceName:     "cipher-test",
		OtelSdkDisabled: "true",
	})
	if err != nil {
		t.Fatalf("telemetry setup: %v", err)
	}

	opts = append([]AgentOption{WithTelemetry(tel)}, opts...)
	resolver := scriptedResolver{client: llm.NewObservedLLM(client, tel)}
	agent, err := NewAgent(resolver, registry, opts...)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	return agent
}

// A failed tool must come back to the model as an is_error result. Dropping it
// leaves the assistant's tool_use block unanswered, which the provider rejects
// on the following turn, and denies the model any chance to correct itself.
func TestRunReturnsToolFailureToModel(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.RegisterTool(failingTool{name: "execute_sql"})

	client := &scriptedLLM{responses: []sharedModel.ChatResponse{
		toolUseResponse("call_1", "execute_sql"),
		endTurnResponse("I hit an error reading that data."),
	}}

	agent := newTestAgent(t, client, registry)

	res, err := agent.Run(testContext(t), sharedModel.ChatRequest{
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "how much did I spend?"}}},
		},
	}, WithUpdateMetadata(false), WithRunMemoryEnabled(false))
	if err != nil {
		t.Fatalf("Run returned error, want graceful recovery: %v", err)
	}
	if res == nil {
		t.Fatal("Run returned no response")
	}

	if client.calls != 2 {
		t.Fatalf("llm call count = %d, want 2 (the model must get a chance to react)", client.calls)
	}

	// The second request must carry a tool result answering call_1.
	second := client.requests[1]
	var found *sharedModel.ToolResult
	for _, msg := range second.Messages {
		if msg.ToolResult != nil && msg.ToolResult.ToolCallId == "call_1" {
			found = msg.ToolResult
		}
	}
	if found == nil {
		t.Fatalf("no tool result for call_1 in the follow-up request: %#v", second.Messages)
	}
	if !found.IsError {
		t.Error("tool result should be marked IsError so the model knows the call failed")
	}
	if len(found.Content) == 0 || found.Content[0].Text == "" {
		t.Error("tool result should carry the error text so the model can correct itself")
	}
}

// Every tool_use must have a matching tool_result in the next request,
// regardless of how the tool behaved.
func TestRunNeverLeavesToolCallUnanswered(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.RegisterTool(failingTool{name: "execute_sql"})

	client := &scriptedLLM{responses: []sharedModel.ChatResponse{
		toolUseResponse("call_1", "execute_sql"),
		toolUseResponse("call_2", "missing_tool"),
		endTurnResponse("done"),
	}}

	agent := newTestAgent(t, client, registry)

	if _, err := agent.Run(testContext(t), sharedModel.ChatRequest{
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "hi"}}},
		},
	}, WithUpdateMetadata(false), WithRunMemoryEnabled(false)); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	last := client.requests[len(client.requests)-1]
	answered := map[string]bool{}
	for _, msg := range last.Messages {
		if msg.ToolResult != nil {
			answered[msg.ToolResult.ToolCallId] = true
		}
	}
	for _, msg := range last.Messages {
		for _, call := range msg.ToolCalls {
			if !answered[call.ID] {
				t.Fatalf("tool call %q left unanswered: %#v", call.ID, last.Messages)
			}
		}
	}
}

// Exhausting the tool budget should still answer the user rather than surfacing
// an error bubble.
func TestRunExhaustingToolBudgetStillAnswers(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.RegisterTool(failingTool{name: "execute_sql"})

	client := &scriptedLLM{responses: []sharedModel.ChatResponse{
		toolUseResponse("call_1", "execute_sql"),
		toolUseResponse("call_2", "execute_sql"),
		endTurnResponse("Here is what I could determine."),
	}}

	agent := newTestAgent(t, client, registry, WithMaxToolCalls(1))

	res, err := agent.Run(testContext(t), sharedModel.ChatRequest{
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "hi"}}},
		},
	}, WithUpdateMetadata(false), WithRunMemoryEnabled(false))
	if err != nil {
		t.Fatalf("Run returned error, want a final answer: %v", err)
	}
	if res.StopReason != sharedModel.StopReasonEndTurn {
		t.Errorf("stop reason = %q, want end_turn", res.StopReason)
	}
	if got := messageContentText(res.Message.Content); got != "Here is what I could determine." {
		t.Errorf("final text = %q, want the model's closing answer", got)
	}

	// The final turn must have gone out with tools disabled.
	final := client.requests[len(client.requests)-1]
	if len(final.Tools) != 0 {
		t.Errorf("final request should carry no tools, got %d", len(final.Tools))
	}
}

// Exhausting the turn budget takes the same graceful path.
func TestRunExhaustingTurnBudgetStillAnswers(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.RegisterTool(failingTool{name: "execute_sql"})

	client := &scriptedLLM{responses: []sharedModel.ChatResponse{
		toolUseResponse("call_1", "execute_sql"),
		toolUseResponse("call_2", "execute_sql"),
		endTurnResponse("Partial answer."),
	}}

	agent := newTestAgent(t, client, registry, WithMaxTurns(2))

	res, err := agent.Run(testContext(t), sharedModel.ChatRequest{
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "hi"}}},
		},
	}, WithUpdateMetadata(false), WithRunMemoryEnabled(false))
	if err != nil {
		t.Fatalf("Run returned error, want a final answer: %v", err)
	}
	if got := messageContentText(res.Message.Content); got != "Partial answer." {
		t.Errorf("final text = %q", got)
	}
}

// A truncated reply is more useful than an error, so max_tokens with text
// returns that text.
func TestRunMaxTokensReturnsPartialAnswer(t *testing.T) {
	client := &scriptedLLM{responses: []sharedModel.ChatResponse{{
		Message: sharedModel.AgentMessage{
			Role:    sharedModel.RoleAssistant,
			Content: []sharedModel.ContentBlock{{Type: "text", Text: "You spent ₹42 on"}},
		},
		StopReason: sharedModel.StopReasonMaxTokens,
	}}}

	agent := newTestAgent(t, client, tools.NewToolRegistry())

	res, err := agent.Run(testContext(t), sharedModel.ChatRequest{
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "hi"}}},
		},
	}, WithUpdateMetadata(false), WithRunMemoryEnabled(false))
	if err != nil {
		t.Fatalf("Run returned error, want the partial answer: %v", err)
	}
	if got := messageContentText(res.Message.Content); got != "You spent ₹42 on" {
		t.Errorf("text = %q, want the truncated answer", got)
	}
}

// The static half of the system prompt carries the cache breakpoint; the
// dynamic half must not.
func TestRunSplitsSystemPromptForCaching(t *testing.T) {
	client := &scriptedLLM{responses: []sharedModel.ChatResponse{endTurnResponse("hello")}}
	agent := newTestAgent(t, client, tools.NewToolRegistry())

	_, err := agent.Run(testContext(t), sharedModel.ChatRequest{
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "hi"}}},
		},
	},
		WithUpdateMetadata(false),
		WithRunMemoryEnabled(false),
		WithSystemPrompt(SystemPrompt{
			Static:  "stable instructions",
			Dynamic: "today is %s",
			Args:    []any{"2026-07-27"},
		}),
	)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	sent := client.requests[0].Messages
	if len(sent) == 0 || sent[0].Role != sharedModel.RoleSystem {
		t.Fatalf("first message should be the system prompt: %#v", sent)
	}
	blocks := sent[0].Content
	if len(blocks) != 2 {
		t.Fatalf("system block count = %d, want 2 (static then dynamic)", len(blocks))
	}
	if !blocks[0].Cacheable {
		t.Error("static system block should be marked cacheable")
	}
	if blocks[1].Cacheable {
		t.Error("dynamic system block must not be marked cacheable")
	}
	if blocks[1].Text != "today is 2026-07-27" {
		t.Errorf("dynamic block = %q, want the formatted text", blocks[1].Text)
	}
}
