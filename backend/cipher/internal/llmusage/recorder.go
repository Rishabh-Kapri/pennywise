package llmusage

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
)

// Recorder saves one row per model call (llm_usage). The clients
// internal/resolver builds call it, so callers never record usage themselves:
// they only label what a call is for with WithPurpose.
type Recorder interface {
	Record(ctx context.Context, usage sharedModel.LLMUsage)
}

// recordTimeout bounds the insert. Recording runs inside the call, so a slow
// database must not hold up the model's answer for long.
const recordTimeout = 3 * time.Second

// StoreRecorder writes rows through the llm_usage repository. A failed write is
// logged and dropped: usage is never a reason to fail the call it describes.
type StoreRecorder struct {
	repo repository.LLMUsageRepository
}

func NewStoreRecorder(repo repository.LLMUsageRepository) *StoreRecorder {
	return &StoreRecorder{repo: repo}
}

func (r *StoreRecorder) Record(ctx context.Context, usage sharedModel.LLMUsage) {
	if r == nil || r.repo == nil {
		return
	}
	// The call's own context may already be cancelled (that's often why it
	// failed); the row must still be written.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	if err := r.repo.Record(writeCtx, usage); err != nil {
		logger.Logger(ctx).Error("recording llm usage failed",
			"purpose", usage.Purpose, "provider", usage.Provider, "model", usage.Model, "error", err)
	}
}

type purposeKey struct{}
type runKey struct{}
type promptVersionKey struct{}

// WithPurpose labels the model calls made under ctx (llm_usage.purpose).
func WithPurpose(ctx context.Context, purpose sharedModel.LLMUsagePurpose) context.Context {
	return context.WithValue(ctx, purposeKey{}, purpose)
}

// PurposeFrom returns the label WithPurpose set, or fallback.
func PurposeFrom(ctx context.Context, fallback sharedModel.LLMUsagePurpose) sharedModel.LLMUsagePurpose {
	if purpose, ok := ctx.Value(purposeKey{}).(sharedModel.LLMUsagePurpose); ok && purpose != "" {
		return purpose
	}
	return fallback
}

// WithAgentRun links the model calls made under ctx to an agent run.
func WithAgentRun(ctx context.Context, runID uuid.UUID) context.Context {
	if runID == uuid.Nil {
		return ctx
	}
	return context.WithValue(ctx, runKey{}, runID)
}

// AgentRunFrom returns the run WithAgentRun set, if any.
func AgentRunFrom(ctx context.Context) (uuid.UUID, bool) {
	runID, ok := ctx.Value(runKey{}).(uuid.UUID)
	return runID, ok && runID != uuid.Nil
}

// WithPromptVersion records which prompt the calls made under ctx use.
func WithPromptVersion(ctx context.Context, version string) context.Context {
	if version == "" {
		return ctx
	}
	return context.WithValue(ctx, promptVersionKey{}, version)
}

// PromptVersionFrom returns the version WithPromptVersion set, or "".
func PromptVersionFrom(ctx context.Context) string {
	version, _ := ctx.Value(promptVersionKey{}).(string)
	return version
}
