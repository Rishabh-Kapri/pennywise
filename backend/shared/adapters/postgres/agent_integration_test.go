package postgres

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAgentUpdateConversation(t *testing.T) {
	pool := testPool(t)
	s := newSeed(t, pool)
	ctx := context.Background()
	userID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO auth_users (id) VALUES ($1)`, userID)
	require.NoError(t, err)

	repo := NewAgentRepository(pool)
	original := "New chat"
	conv, err := repo.CreateConversation(ctx, repository.CreateAgentConversationParams{
		UserID: userID, BudgetID: s.budgetID, AgentKey: "chat", Title: &original,
	})
	require.NoError(t, err)

	title := func() string {
		t.Helper()
		got, err := repo.GetConversation(ctx, conv.ID, userID, s.budgetID)
		require.NoError(t, err)
		require.NotNil(t, got.Title)
		return *got.Title
	}

	t.Run("a title is saved", func(t *testing.T) {
		renamed := "Ways I can help"
		require.NoError(t, repo.UpdateConversation(ctx, conv.ID, model.AgentConversation{Title: &renamed}))
		require.Equal(t, renamed, title())
	})
	t.Run("an update with nothing set changes nothing", func(t *testing.T) {
		before := title()
		require.NoError(t, repo.UpdateConversation(ctx, conv.ID, model.AgentConversation{}))
		require.Equal(t, before, title())
	})
	t.Run("a deleted conversation is not updated", func(t *testing.T) {
		before := title()
		require.NoError(t, repo.DeleteConversation(ctx, conv.ID, userID, s.budgetID))
		other := "after delete"
		require.NoError(t, repo.UpdateConversation(ctx, conv.ID, model.AgentConversation{Title: &other}))
		_, err := pool.Exec(ctx, `UPDATE conversations SET deleted = NULL WHERE id = $1`, conv.ID)
		require.NoError(t, err)
		require.Equal(t, before, title())
	})
}

func TestAgentRecordRunResultRollsUpUsage(t *testing.T) {
	pool := testPool(t)
	s := newSeed(t, pool)
	ctx := context.Background()
	userID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO auth_users (id) VALUES ($1)`, userID)
	require.NoError(t, err)

	repo := NewAgentRepository(pool)
	usage := NewLLMUsageRepository(pool)
	conv, err := repo.CreateConversation(ctx, repository.CreateAgentConversationParams{
		UserID: userID, BudgetID: s.budgetID, AgentKey: "chat",
	})
	require.NoError(t, err)
	run, err := repo.CreateRun(ctx, repository.CreateAgentRunParams{
		UserID: userID, BudgetID: s.budgetID, AgentKey: "chat", ConversationID: conv.ID,
	})
	require.NoError(t, err)
	require.Nil(t, run.InputTokens, "a new run has no metrics yet")

	ip := func(v int) *int { return &v }
	cp := func(v int64) *int64 { return &v }
	str := func(v string) *string { return &v }
	row := func(purpose model.LLMUsagePurpose, provider, mdl string, in, out, read, write int, cost *int64) model.LLMUsage {
		return model.LLMUsage{
			BudgetID: &s.budgetID, UserID: &userID, AgentRunID: &run.ID, Purpose: purpose,
			Provider: provider, Model: mdl, InputTokens: ip(in), OutputTokens: ip(out),
			CacheReadTokens: ip(read), CacheWriteTokens: ip(write), CostMicros: cost, LatencyMs: 900,
			Status: model.LLMUsageStatusOK, StopReason: str("tool_use"),
		}
	}
	require.NoError(t, usage.Record(ctx, row(model.LLMPurposeAgentTurn, "anthropic", "claude-a", 1000, 50, 0, 900, cp(4000))))
	require.NoError(t, usage.Record(ctx, row(model.LLMPurposeAgentTurn, "openrouter", "claude-b", 1200, 80, 900, 0, nil)))
	// A failed turn reported nothing; it still counts as a turn.
	require.NoError(t, usage.Record(ctx, model.LLMUsage{
		BudgetID: &s.budgetID, AgentRunID: &run.ID, Purpose: model.LLMPurposeAgentTurn,
		Provider: "openrouter", Model: "claude-b", LatencyMs: 30, Status: model.LLMUsageStatusError, Error: str("529"),
	}))
	// The title isn't one of the run's turns.
	require.NoError(t, usage.Record(ctx, row(model.LLMPurposeTitle, "openai", "gpt-title", 30, 5, 0, 0, cp(10))))

	invocation := func(seq int, name string) model.ToolInvocation {
		ms := 12
		return model.ToolInvocation{
			Sequence: seq, Turn: 1, ToolCallID: "call_" + name, ToolName: name,
			Input: []byte(`{"q":1}`), Output: []byte(`{"Content":[]}`), InputBytes: 7, OutputBytes: 14, DurationMs: &ms,
		}
	}
	result := repository.RecordAgentRunResult{
		RunID: run.ID, BudgetID: s.budgetID, StopReason: "end_turn", PromptVersion: "sha256:abc",
		ToolInvocations: []model.ToolInvocation{invocation(1, "execute_sql"), invocation(2, "get_today")},
	}
	require.NoError(t, repo.RecordRunResult(ctx, result))
	// Recording again (a retry) replaces the invocations instead of adding them.
	require.NoError(t, repo.RecordRunResult(ctx, result))

	got, err := repo.GetRun(ctx, userID, s.budgetID, run.ID)
	require.NoError(t, err)
	require.Equal(t, "end_turn", *got.StopReason)
	require.Equal(t, "sha256:abc", *got.PromptVersion)
	require.Equal(t, 2200, *got.InputTokens)
	require.Equal(t, 130, *got.OutputTokens)
	require.Equal(t, 900, *got.CacheReadTokens)
	require.Equal(t, 900, *got.CacheWriteTokens)
	require.Equal(t, int64(4000), *got.CostMicros, "the sum of what was reported")
	require.Equal(t, 3, *got.TurnCount)
	require.Equal(t, 2, *got.ToolCallCount)
	require.Equal(t, "openrouter", *got.ResolvedProvider, "the last turn's")
	require.Equal(t, "claude-b", *got.ResolvedModel)

	runs, err := repo.GetAllConversationRuns(ctx, conv.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Len(t, runs[0].ToolInvocations, 2)
	require.Equal(t, "execute_sql", runs[0].ToolInvocations[0].ToolName)
	require.JSONEq(t, `{"q":1}`, string(runs[0].ToolInvocations[0].Input))

	// The two queries the PR promises.
	var costMicros int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT cost_micros FROM agent_runs WHERE id = $1`, run.ID).Scan(&costMicros))
	require.Equal(t, int64(4000), costMicros)
	var hitRatio float64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT sum(cache_read_tokens)::float / NULLIF(sum(input_tokens), 0)
		FROM llm_usage WHERE budget_id = $1 AND purpose = 'agent_turn'`, s.budgetID).Scan(&hitRatio))
	require.InDelta(t, 900.0/2200.0, hitRatio, 1e-9)

	t.Run("an unknown run is not found", func(t *testing.T) {
		err := repo.RecordRunResult(ctx, repository.RecordAgentRunResult{RunID: uuid.New(), BudgetID: s.budgetID})
		require.ErrorIs(t, err, repository.ErrNotFound)
	})
	t.Run("another budget's run is not found", func(t *testing.T) {
		err := repo.RecordRunResult(ctx, repository.RecordAgentRunResult{RunID: run.ID, BudgetID: uuid.New()})
		require.ErrorIs(t, err, repository.ErrNotFound)
	})
	t.Run("deleting the conversation deletes its tool invocations, not the usage", func(t *testing.T) {
		require.NoError(t, repo.DeleteConversation(ctx, conv.ID, userID, s.budgetID))
		var invocations, usageRows int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM tool_invocations WHERE run_id = $1`, run.ID).Scan(&invocations))
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM llm_usage WHERE agent_run_id = $1`, run.ID).Scan(&usageRows))
		require.Zero(t, invocations)
		require.Equal(t, 4, usageRows)
	})
}

func TestLLMUsageRecordWithoutBudgetOrUsage(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewLLMUsageRepository(pool)
	require.NoError(t, repo.Record(ctx, model.LLMUsage{Provider: "ollama", Model: "qwen", LatencyMs: -3}))

	var purpose, status string
	var latency int
	var input *int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT purpose, status, latency_ms, input_tokens FROM llm_usage WHERE budget_id IS NULL AND model = 'qwen'`,
	).Scan(&purpose, &status, &latency, &input))
	require.Equal(t, "other", purpose)
	require.Equal(t, "ok", status)
	require.Zero(t, latency)
	require.Nil(t, input)
}
