package postgres

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type llmUsageRepo struct {
	BaseRepository
}

func NewLLMUsageRepository(pool *pgxpool.Pool) repository.LLMUsageRepository {
	return &llmUsageRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *llmUsageRepo) Record(ctx context.Context, u model.LLMUsage) error {
	purpose := u.Purpose
	if purpose == "" {
		purpose = model.LLMPurposeOther
	}
	status := u.Status
	if status == "" {
		status = model.LLMUsageStatusOK
	}
	latency := u.LatencyMs
	if latency < 0 {
		latency = 0
	}
	_, err := r.Executor(ctx).Exec(
		ctx,
		`INSERT INTO llm_usage (
			budget_id, user_id, agent_run_id, purpose, provider, model, prompt_version,
			input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_micros,
			latency_ms, stop_reason, status, error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		u.BudgetID, u.UserID, u.AgentRunID, string(purpose), u.Provider, u.Model, u.PromptVersion,
		u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens, u.CostMicros,
		latency, u.StopReason, string(status), u.Error,
	)
	return err
}
