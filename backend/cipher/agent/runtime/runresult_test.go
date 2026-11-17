package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/tools"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/llmusage"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeRunStore struct {
	results chan repository.RecordAgentRunResult
}

func newFakeRunStore() *fakeRunStore {
	return &fakeRunStore{results: make(chan repository.RecordAgentRunResult, 1)}
}

func (f *fakeRunStore) RecordRunResult(_ context.Context, r repository.RecordAgentRunResult) error {
	f.results <- r
	return nil
}

func (f *fakeRunStore) wait(t *testing.T) repository.RecordAgentRunResult {
	t.Helper()
	select {
	case r := <-f.results:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("run result was never recorded")
		return repository.RecordAgentRunResult{}
	}
}

// ctxLLM remembers the context of each call, to check what the usage
// recorder will see.
type ctxLLM struct {
	scriptedLLM
	purposes []sharedModel.LLMUsagePurpose
	versions []string
}

func (c *ctxLLM) Chat(ctx context.Context, req sharedModel.ChatRequest, id *string) (*sharedModel.ChatResponse, error) {
	c.purposes = append(c.purposes, llmusage.PurposeFrom(ctx, ""))
	c.versions = append(c.versions, llmusage.PromptVersionFrom(ctx))
	return c.scriptedLLM.Chat(ctx, req, id)
}

func userAsks(runID uuid.UUID) sharedModel.ChatRequest {
	return sharedModel.ChatRequest{
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "how much did I spend?"}}},
		},
		Metadata: map[string]string{"runId": runID.String()},
	}
}

func TestRunRecordsResultAndToolInvocations(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.RegisterTool(failingTool{name: "execute_sql"})
	client := &ctxLLM{scriptedLLM: scriptedLLM{responses: []sharedModel.ChatResponse{
		toolUseResponse("call_1", "execute_sql"),
		endTurnResponse("I hit an error reading that data."),
	}}}
	store := newFakeRunStore()
	agent := newTestAgent(t, client, registry, WithRunStore(store))

	runID := uuid.New()
	_, err := agent.Run(testContext(t), userAsks(runID),
		WithSystemPrompt(SystemPrompt{Static: "static", Dynamic: "today is %s", Args: []any{"2026-11-17"}}),
		WithRunMemoryEnabled(false))
	require.NoError(t, err)

	got := store.wait(t)
	require.Equal(t, runID, got.RunID)
	require.Equal(t, "end_turn", got.StopReason)
	require.True(t, strings.HasPrefix(got.PromptVersion, "sha256:"))
	require.Len(t, got.ToolInvocations, 1)
	inv := got.ToolInvocations[0]
	require.Equal(t, 1, inv.Sequence)
	require.Equal(t, 1, inv.Turn)
	require.Equal(t, "call_1", inv.ToolCallID)
	require.Equal(t, "execute_sql", inv.ToolName)
	require.True(t, inv.IsError)
	require.JSONEq(t, `{}`, string(inv.Input))
	require.Contains(t, string(inv.Output), "does not exist")
	require.NotNil(t, inv.DurationMs)

	// Every model turn is labelled for llm_usage with the run's prompt version.
	require.Equal(t, []sharedModel.LLMUsagePurpose{sharedModel.LLMPurposeAgentTurn, sharedModel.LLMPurposeAgentTurn}, client.purposes)
	require.Equal(t, []string{got.PromptVersion, got.PromptVersion}, client.versions)
}

func TestRunRecordsToolBudgetStop(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.RegisterTool(failingTool{name: "execute_sql"})
	twoCalls := toolUseResponse("call_1", "execute_sql")
	twoCalls.Message.ToolCalls = append(twoCalls.Message.ToolCalls,
		sharedModel.ToolCall{ID: "call_2", Name: "execute_sql", Arguments: json.RawMessage(`{}`)})
	client := &scriptedLLM{responses: []sharedModel.ChatResponse{twoCalls, endTurnResponse("Here's what I have.")}}
	store := newFakeRunStore()
	agent := newTestAgent(t, client, registry, WithRunStore(store), WithMaxToolCalls(1))

	_, err := agent.Run(testContext(t), userAsks(uuid.New()), WithRunMemoryEnabled(false))
	require.NoError(t, err)

	got := store.wait(t)
	require.Equal(t, sharedModel.RunStopReasonMaxToolCalls, got.StopReason)
	require.Empty(t, got.ToolInvocations, "refused calls never ran")
}

func TestRunRecordsErrorStop(t *testing.T) {
	client := &scriptedLLM{} // no scripted answer: the first call fails
	store := newFakeRunStore()
	agent := newTestAgent(t, client, tools.NewToolRegistry(), WithRunStore(store))

	_, err := agent.Run(testContext(t), userAsks(uuid.New()), WithRunMemoryEnabled(false))
	require.Error(t, err)
	require.Equal(t, "error", store.wait(t).StopReason)
}

func TestRunWithoutMetadataUpdateRecordsNothing(t *testing.T) {
	store := newFakeRunStore()
	client := &scriptedLLM{responses: []sharedModel.ChatResponse{endTurnResponse("hi")}}
	agent := newTestAgent(t, client, tools.NewToolRegistry(), WithRunStore(store))
	_, err := agent.Run(testContext(t), userAsks(uuid.New()), WithUpdateMetadata(false), WithRunMemoryEnabled(false))
	require.NoError(t, err)
	select {
	case <-store.results:
		t.Fatal("recorded a run that asked not to be")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBoundToolJSON(t *testing.T) {
	small, size, cut := boundToolJSON([]byte(`{"rows":[1,2]}`))
	require.JSONEq(t, `{"rows":[1,2]}`, string(small))
	require.Equal(t, 14, size)
	require.False(t, cut)

	invalid, _, cut := boundToolJSON([]byte(`{"from": "2026-`))
	require.False(t, cut)
	require.Equal(t, `"{\"from\": \"2026-"`, string(invalid), "malformed arguments are kept as a string")

	nul, _, _ := boundToolJSON([]byte(`{"text":"a\u0000b"}`))
	require.NotContains(t, string(nul), `\u0000`, "jsonb refuses NUL")

	big := `{"rows":"` + strings.Repeat("é", toolPayloadLimit) + `"}`
	stored, size, cut := boundToolJSON([]byte(big))
	require.True(t, cut)
	require.Equal(t, len(big), size)
	require.LessOrEqual(t, len(stored), toolPayloadLimit)
	require.True(t, json.Valid(stored))
	preview, ok := TruncatedPreview(stored)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(big, preview))
}

func TestPromptVersionChangesWithPromptOrTools(t *testing.T) {
	base := PromptVersion(SystemPrompt{Static: "a", Dynamic: "today %s"}, []sharedModel.ToolDefiniton{{Name: "x"}})
	require.Equal(t, base, PromptVersion(SystemPrompt{Static: "a", Dynamic: "today %s", Args: []any{"now"}},
		[]sharedModel.ToolDefiniton{{Name: "x"}}), "the filled-in values don't change the version")
	require.NotEqual(t, base, PromptVersion(SystemPrompt{Static: "b", Dynamic: "today %s"}, []sharedModel.ToolDefiniton{{Name: "x"}}))
	require.NotEqual(t, base, PromptVersion(SystemPrompt{Static: "a", Dynamic: "today %s"}, []sharedModel.ToolDefiniton{{Name: "y"}}))
}
