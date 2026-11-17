package llmusage

import (
	"context"
	"errors"
	"sync"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeRecorder struct {
	mu   sync.Mutex
	rows []sharedModel.LLMUsage
}

func (f *fakeRecorder) Record(_ context.Context, u sharedModel.LLMUsage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, u)
}

func (f *fakeRecorder) only(t *testing.T) sharedModel.LLMUsage {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.rows, 1)
	return f.rows[0]
}

type fakeLLM struct {
	res    *sharedModel.ChatResponse
	err    error
	chunks []sharedModel.StreamChunk
}

func (f *fakeLLM) Chat(context.Context, sharedModel.ChatRequest, *string) (*sharedModel.ChatResponse, error) {
	return f.res, f.err
}

func (f *fakeLLM) Stream(context.Context, sharedModel.ChatRequest, *string) <-chan sharedModel.StreamChunk {
	ch := make(chan sharedModel.StreamChunk, len(f.chunks))
	for _, c := range f.chunks {
		ch <- c
	}
	close(ch)
	return ch
}

type fakeEmbedder struct{ err error }

func (f fakeEmbedder) Embed(context.Context, string, string) ([]float64, error) {
	return []float64{1}, f.err
}

type fakeDecisions struct {
	res *sharedModel.DecisionResponse
	err error
}

func (f fakeDecisions) Ask(context.Context, sharedModel.DecisionRequest) (*sharedModel.DecisionResponse, error) {
	return f.res, f.err
}

func intPtr(v int) *int { return &v }

func callCtx(budgetID, runID uuid.UUID) context.Context {
	ctx := utils.WithBudgetID(context.Background(), budgetID)
	ctx = WithAgentRun(ctx, runID)
	ctx = WithPromptVersion(ctx, "sha256:abc")
	return WithPurpose(ctx, sharedModel.LLMPurposeAgentTurn)
}

func TestChatSuccessRecordsUsage(t *testing.T) {
	budgetID, runID, owner := uuid.New(), uuid.New(), uuid.New()
	cost := 0.0123456
	rec := &fakeRecorder{}
	client := WrapLLM(&fakeLLM{res: &sharedModel.ChatResponse{
		Model:      "claude-x-20260101",
		StopReason: sharedModel.StopReasonEndTurn,
		Usage: sharedModel.Usage{
			Available: true, InputTokens: 1200, OutputTokens: 80,
			CacheReadTokens: 1000, CacheWriteTokens: 150, Cost: &cost,
		},
	}}, Key{Provider: sharedModel.LLMProviderOpenRouter, OwnerID: owner}, rec)

	_, err := client.Chat(callCtx(budgetID, runID), sharedModel.ChatRequest{Model: "claude-x"}, nil)
	require.NoError(t, err)

	row := rec.only(t)
	require.Equal(t, sharedModel.LLMPurposeAgentTurn, row.Purpose)
	require.Equal(t, "openrouter", row.Provider)
	require.Equal(t, "claude-x-20260101", row.Model, "the model that answered wins over the one asked for")
	require.Equal(t, budgetID, *row.BudgetID)
	require.Equal(t, owner, *row.UserID)
	require.Equal(t, runID, *row.AgentRunID)
	require.Equal(t, "sha256:abc", *row.PromptVersion)
	require.Equal(t, intPtr(1200), row.InputTokens)
	require.Equal(t, intPtr(80), row.OutputTokens)
	require.Equal(t, intPtr(1000), row.CacheReadTokens)
	require.Equal(t, intPtr(150), row.CacheWriteTokens)
	require.Equal(t, int64(12346), *row.CostMicros)
	require.Equal(t, "end_turn", *row.StopReason)
	require.Equal(t, sharedModel.LLMUsageStatusOK, row.Status)
	require.Nil(t, row.Error)
}

func TestChatErrorStillRecords(t *testing.T) {
	rec := &fakeRecorder{}
	boom := errors.New("upstream 529 overloaded")
	client := WrapLLM(&fakeLLM{err: boom}, Key{Provider: sharedModel.LLMProviderAnthropic}, rec)

	ctx := WithPurpose(utils.WithBudgetID(context.Background(), uuid.New()), sharedModel.LLMPurposePipelineExtract)
	_, err := client.Chat(ctx, sharedModel.ChatRequest{Model: "claude-x"}, nil)
	require.ErrorIs(t, err, boom)

	row := rec.only(t)
	require.Equal(t, sharedModel.LLMPurposePipelineExtract, row.Purpose)
	require.Equal(t, "claude-x", row.Model, "the requested model when nothing answered")
	require.Equal(t, sharedModel.LLMUsageStatusError, row.Status)
	require.Equal(t, "upstream 529 overloaded", *row.Error)
	require.Nil(t, row.InputTokens)
	require.Nil(t, row.CostMicros)
	require.Nil(t, row.UserID, "no owner given, none stored")
}

func TestChatErrorIsCutShort(t *testing.T) {
	rec := &fakeRecorder{}
	long := make([]byte, 2000)
	for i := range long {
		long[i] = 'x'
	}
	client := WrapLLM(&fakeLLM{err: errors.New(string(long))}, Key{Provider: sharedModel.LLMProviderOpenAI}, rec)
	_, _ = client.Chat(context.Background(), sharedModel.ChatRequest{Model: "m"}, nil)
	require.Len(t, *rec.only(t).Error, errorLimit)
}

func TestChatWithoutUsageLeavesCountsNull(t *testing.T) {
	rec := &fakeRecorder{}
	client := WrapLLM(&fakeLLM{res: &sharedModel.ChatResponse{StopReason: sharedModel.StopReasonEndTurn}},
		Key{Provider: sharedModel.LLMProviderOpenAI}, rec)

	_, err := client.Chat(context.Background(), sharedModel.ChatRequest{Model: "gpt-x"}, nil)
	require.NoError(t, err)

	row := rec.only(t)
	require.Equal(t, sharedModel.LLMPurposeOther, row.Purpose, "an unlabelled call is still recorded")
	require.Nil(t, row.InputTokens)
	require.Nil(t, row.OutputTokens)
	require.Nil(t, row.CacheReadTokens)
	require.Nil(t, row.CostMicros)
	require.Nil(t, row.BudgetID)
	require.Nil(t, row.AgentRunID)
	require.Equal(t, sharedModel.LLMUsageStatusOK, row.Status)
}

func TestLocalModelCostsNothing(t *testing.T) {
	rec := &fakeRecorder{}
	// Ollama counts tokens without marking usage available.
	client := WrapLLM(&fakeLLM{res: &sharedModel.ChatResponse{
		StopReason: sharedModel.StopReasonEndTurn,
		Usage:      sharedModel.Usage{InputTokens: 40, OutputTokens: 5},
	}}, Key{Provider: sharedModel.LLMProviderOllama}, rec)

	_, err := client.Chat(context.Background(), sharedModel.ChatRequest{Model: "qwen"}, nil)
	require.NoError(t, err)
	row := rec.only(t)
	require.Equal(t, intPtr(40), row.InputTokens)
	require.Equal(t, int64(0), *row.CostMicros)
}

func TestStreamRecordsCompletionBeforeClosing(t *testing.T) {
	rec := &fakeRecorder{}
	client := WrapLLM(&fakeLLM{chunks: []sharedModel.StreamChunk{
		{Type: sharedModel.ChunkEventText, Text: "hello"},
		{
			Type: sharedModel.ChunkEventCompleted, Model: "gpt-x-2026",
			StopReason: sharedModel.StopReasonToolUse,
			Usage:      sharedModel.Usage{Available: true, InputTokens: 10, OutputTokens: 2},
		},
	}}, Key{Provider: sharedModel.LLMProviderOpenAI}, rec)

	var got []sharedModel.StreamChunk
	for chunk := range client.Stream(context.Background(), sharedModel.ChatRequest{Model: "gpt-x"}, nil) {
		got = append(got, chunk)
	}
	require.Len(t, got, 2, "every chunk is passed on")

	row := rec.only(t) // already written when the stream closed
	require.Equal(t, "gpt-x-2026", row.Model)
	require.Equal(t, "tool_use", *row.StopReason)
	require.Equal(t, intPtr(10), row.InputTokens)
	require.Equal(t, sharedModel.LLMUsageStatusOK, row.Status)
}

func TestStreamErrorChunkRecordsError(t *testing.T) {
	rec := &fakeRecorder{}
	client := WrapLLM(&fakeLLM{chunks: []sharedModel.StreamChunk{
		{Type: sharedModel.ChunkEventError, Text: "rate limited"},
	}}, Key{Provider: sharedModel.LLMProviderOpenAI}, rec)

	for range client.Stream(context.Background(), sharedModel.ChatRequest{Model: "gpt-x"}, nil) {
	}
	row := rec.only(t)
	require.Equal(t, sharedModel.LLMUsageStatusError, row.Status)
	require.Equal(t, "rate limited", *row.Error)
	require.Nil(t, row.InputTokens)
}

func TestStreamWithoutCompletionIsAnError(t *testing.T) {
	rec := &fakeRecorder{}
	client := WrapLLM(&fakeLLM{chunks: []sharedModel.StreamChunk{
		{Type: sharedModel.ChunkEventText, Text: "cut"},
	}}, Key{Provider: sharedModel.LLMProviderOpenAI}, rec)

	for range client.Stream(context.Background(), sharedModel.ChatRequest{Model: "gpt-x"}, nil) {
	}
	require.Equal(t, sharedModel.LLMUsageStatusError, rec.only(t).Status)
}

func TestCancelledCallIsMarkedCancelled(t *testing.T) {
	rec := &fakeRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := WrapLLM(&fakeLLM{err: context.Canceled}, Key{Provider: sharedModel.LLMProviderOpenAI}, rec)
	_, _ = client.Chat(ctx, sharedModel.ChatRequest{Model: "gpt-x"}, nil)
	require.Equal(t, sharedModel.LLMUsageStatusCancelled, rec.only(t).Status)
}

func TestEmbedderRecordsAsEmbedding(t *testing.T) {
	rec := &fakeRecorder{}
	runID := uuid.New()
	embedder := WrapEmbedder(fakeEmbedder{}, Key{Provider: sharedModel.LLMProviderOpenRouter}, rec)

	_, err := embedder.Embed(callCtx(uuid.New(), runID), "embed-1024", "text")
	require.NoError(t, err)

	row := rec.only(t)
	require.Equal(t, sharedModel.LLMPurposeEmbedding, row.Purpose, "not the agent turn it was made in")
	require.Equal(t, runID, *row.AgentRunID)
	require.Equal(t, "embed-1024", row.Model)
	require.Nil(t, row.InputTokens)
}

func TestEmbedderErrorRecords(t *testing.T) {
	rec := &fakeRecorder{}
	embedder := WrapEmbedder(fakeEmbedder{err: errors.New("dimension mismatch")},
		Key{Provider: sharedModel.LLMProviderOllama}, rec)
	_, err := embedder.Embed(context.Background(), "m", "t")
	require.Error(t, err)
	require.Equal(t, sharedModel.LLMUsageStatusError, rec.only(t).Status)
}

func TestDecisionsRecordUsage(t *testing.T) {
	rec := &fakeRecorder{}
	cost := 0.0002
	d := WrapDecisions(fakeDecisions{res: &sharedModel.DecisionResponse{
		Model: "jev-latest",
		Usage: sharedModel.Usage{Available: true, InputTokens: 300, OutputTokens: 4, Cost: &cost},
	}}, Key{Provider: sharedModel.LLMProviderTypeSafe}, rec)

	_, err := d.Ask(WithPurpose(context.Background(), sharedModel.LLMPurposePipelineVerify),
		sharedModel.DecisionRequest{Model: "jev"})
	require.NoError(t, err)
	row := rec.only(t)
	require.Equal(t, sharedModel.LLMPurposePipelineVerify, row.Purpose)
	require.Equal(t, "jev-latest", row.Model)
	require.Equal(t, int64(200), *row.CostMicros)
}

func TestDecisionsErrorDefaultsPurpose(t *testing.T) {
	rec := &fakeRecorder{}
	d := WrapDecisions(fakeDecisions{err: errors.New("bad gateway")}, Key{Provider: sharedModel.LLMProviderTypeSafe}, rec)
	_, err := d.Ask(context.Background(), sharedModel.DecisionRequest{Model: "jev"})
	require.Error(t, err)
	row := rec.only(t)
	require.Equal(t, sharedModel.LLMPurposeDecision, row.Purpose)
	require.Equal(t, sharedModel.LLMUsageStatusError, row.Status)
}

func TestNoRecorderReturnsClientUnchanged(t *testing.T) {
	inner := &fakeLLM{}
	require.Same(t, inner, WrapLLM(inner, Key{}, nil))
}
