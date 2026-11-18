package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// LLMUsageRepository stores one row per model call (table llm_usage).
type LLMUsageRepository interface {
	Record(ctx context.Context, usage model.LLMUsage) error
	// Summary totals the budget's calls since since, with days counted in
	// the IANA zone tz.
	Summary(ctx context.Context, budgetID uuid.UUID, since time.Time, tz string) (model.LLMUsageSummary, error)
}
