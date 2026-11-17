package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// LLMUsageRepository stores one row per model call (table llm_usage).
type LLMUsageRepository interface {
	Record(ctx context.Context, usage model.LLMUsage) error
}
