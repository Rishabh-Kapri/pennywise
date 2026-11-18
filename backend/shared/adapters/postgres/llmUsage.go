package postgres

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
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

// usageTotalsSQL is the aggregate list every Summary query selects, in the
// order scanUsageTotals reads it.
const usageTotalsSQL = `count(*),
	count(*) FILTER (WHERE status <> 'ok'),
	count(*) FILTER (WHERE cost_micros IS NULL),
	COALESCE(sum(input_tokens), 0),
	COALESCE(sum(output_tokens), 0),
	COALESCE(sum(cache_read_tokens), 0),
	COALESCE(sum(cost_micros), 0)`

func usageTotalsDest(t *model.LLMUsageTotals) []any {
	return []any{&t.Calls, &t.FailedCalls, &t.UnpricedCalls, &t.InputTokens, &t.OutputTokens, &t.CacheReadTokens, &t.CostMicros}
}

func (r *llmUsageRepo) Summary(
	ctx context.Context,
	budgetID uuid.UUID,
	since time.Time,
	tz string,
) (model.LLMUsageSummary, error) {
	summary := model.LLMUsageSummary{
		Since: since, Days: []model.LLMUsageDay{}, ByPurpose: []model.LLMUsageGroup{}, ByModel: []model.LLMUsageGroup{},
	}
	exec := r.Executor(ctx)
	if err := exec.QueryRow(ctx,
		`SELECT `+usageTotalsSQL+` FROM llm_usage WHERE budget_id = $1 AND created_at >= $2`,
		budgetID, since,
	).Scan(usageTotalsDest(&summary.Totals)...); err != nil {
		return summary, err
	}
	if summary.Totals.Calls == 0 {
		return summary, nil
	}

	rows, err := exec.Query(ctx, `
		SELECT to_char((created_at AT TIME ZONE $3)::date, 'YYYY-MM-DD') AS day, `+usageTotalsSQL+`
		FROM llm_usage WHERE budget_id = $1 AND created_at >= $2
		GROUP BY day ORDER BY day`, budgetID, since, tz)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var d model.LLMUsageDay
		if err := rows.Scan(append([]any{&d.Date}, usageTotalsDest(&d.LLMUsageTotals)...)...); err != nil {
			rows.Close()
			return summary, err
		}
		summary.Days = append(summary.Days, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return summary, err
	}

	groups := func(keyCols string) ([]model.LLMUsageGroup, error) {
		rows, err := exec.Query(ctx, `
			SELECT `+keyCols+`, `+usageTotalsSQL+`
			FROM llm_usage WHERE budget_id = $1 AND created_at >= $2
			GROUP BY 1, 2 ORDER BY sum(cost_micros) DESC NULLS LAST, count(*) DESC`, budgetID, since)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []model.LLMUsageGroup{}
		for rows.Next() {
			var g model.LLMUsageGroup
			if err := rows.Scan(append([]any{&g.Key, &g.Provider}, usageTotalsDest(&g.LLMUsageTotals)...)...); err != nil {
				return nil, err
			}
			out = append(out, g)
		}
		return out, rows.Err()
	}
	if summary.ByPurpose, err = groups(`purpose, ''`); err != nil {
		return summary, err
	}
	if summary.ByModel, err = groups(`model, provider`); err != nil {
		return summary, err
	}
	return summary, nil
}
