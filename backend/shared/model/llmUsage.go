package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LLMUsageStatus is how a model call ended.
type LLMUsageStatus string

const (
	LLMUsageStatusOK        LLMUsageStatus = "ok"
	LLMUsageStatusError     LLMUsageStatus = "error"
	LLMUsageStatusCancelled LLMUsageStatus = "cancelled"
)

// LLMUsagePurpose says what a model call was for. Stored as text: add values
// here, not in the schema.
type LLMUsagePurpose string

const (
	LLMPurposeAgentTurn         LLMUsagePurpose = "agent_turn"
	LLMPurposeTitle             LLMUsagePurpose = "title"
	LLMPurposeMemoryObserve     LLMUsagePurpose = "memory_observe"
	LLMPurposePipelineCheck     LLMUsagePurpose = "pipeline_check"
	LLMPurposePipelineExtract   LLMUsagePurpose = "pipeline_extract"
	LLMPurposePipelineVerify    LLMUsagePurpose = "pipeline_verify"
	LLMPurposePipelinePredict   LLMUsagePurpose = "pipeline_predict"
	LLMPurposeStatementExtract  LLMUsagePurpose = "statement_extract"
	LLMPurposeStatementFallback LLMUsagePurpose = "statement_fallback"
	LLMPurposeEmbedding         LLMUsagePurpose = "embedding"
	LLMPurposeDecision          LLMUsagePurpose = "decision"
	LLMPurposeModelTest         LLMUsagePurpose = "model_test"
	LLMPurposeCategoryDescribe  LLMUsagePurpose = "category_describe"
	LLMPurposeOther             LLMUsagePurpose = "other"
)

// LLMUsage is one call to a model provider (a row of llm_usage). It never
// holds prompt or response text. Token and cost fields are nil when the
// provider didn't report them.
type LLMUsage struct {
	ID         uuid.UUID       `json:"id"`
	BudgetID   *uuid.UUID      `json:"budgetId,omitempty"`
	UserID     *uuid.UUID      `json:"userId,omitempty"` // the key's owner
	AgentRunID *uuid.UUID      `json:"agentRunId,omitempty"`
	Purpose    LLMUsagePurpose `json:"purpose"`
	Provider   string          `json:"provider"`
	Model      string          `json:"model"`
	// PromptVersion identifies the prompt the call was made with, where the
	// caller knows it (agent runs).
	PromptVersion *string `json:"promptVersion,omitempty"`
	// InputTokens counts every prompt token, cached or not; the cache counts
	// are the part of it read from or written to the prompt cache.
	InputTokens      *int           `json:"inputTokens,omitempty"`
	OutputTokens     *int           `json:"outputTokens,omitempty"`
	CacheReadTokens  *int           `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens *int           `json:"cacheWriteTokens,omitempty"`
	CostMicros       *int64         `json:"costMicros,omitempty"` // millionths of a US dollar
	LatencyMs        int            `json:"latencyMs"`
	StopReason       *string        `json:"stopReason,omitempty"`
	Status           LLMUsageStatus `json:"status"`
	Error            *string        `json:"error,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
}

// ModelUsage totals some model calls (a job's, an agent run's, a
// conversation's). A total is nil when no call reported it: a provider that
// doesn't report cost, a call still running.
type ModelUsage struct {
	Calls        int    `json:"calls"`
	InputTokens  *int64 `json:"inputTokens,omitempty"`
	OutputTokens *int64 `json:"outputTokens,omitempty"`
	// CostMicros is millionths of a US dollar, as the providers reported it.
	CostMicros *int64 `json:"costMicros,omitempty"`
	// Model is the last model that answered.
	Model string `json:"model,omitempty"`
}

// ConversationUsage is what a chat conversation's model calls used: in all,
// and per agent run (one run is one answer, with its title and memory calls).
type ConversationUsage struct {
	Total ModelUsage               `json:"total"`
	Runs  map[uuid.UUID]ModelUsage `json:"runs"`
}

// Agent run stop reasons beyond the model's own StopReason: the run spent a
// budget and was made to answer with what it had.
const (
	RunStopReasonMaxTurns     = "max_turns"
	RunStopReasonMaxToolCalls = "max_tool_calls"
)

// ToolInvocation is one tool call an agent run made (a row of
// tool_invocations). Input and Output are bounded by cipher before they're
// stored; Truncated says one of them was cut.
type ToolInvocation struct {
	ID          uuid.UUID       `json:"id"`
	RunID       uuid.UUID       `json:"runId"`
	Sequence    int             `json:"sequence"`
	Turn        int             `json:"turn"`
	ToolCallID  string          `json:"toolCallId,omitempty"`
	ToolName    string          `json:"toolName"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	IsError     bool            `json:"isError"`
	InputBytes  int             `json:"inputBytes"`
	OutputBytes int             `json:"outputBytes"`
	Truncated   bool            `json:"truncated"`
	DurationMs  *int            `json:"durationMs,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// LLMUsageTotals adds up model calls. CostMicros sums only the calls whose
// provider reported a cost; UnpricedCalls counts the rest (local models count
// as priced at 0).
type LLMUsageTotals struct {
	Calls           int   `json:"calls"`
	FailedCalls     int   `json:"failedCalls"`
	UnpricedCalls   int   `json:"unpricedCalls"`
	InputTokens     int64 `json:"inputTokens"`
	OutputTokens    int64 `json:"outputTokens"`
	CacheReadTokens int64 `json:"cacheReadTokens"`
	CostMicros      int64 `json:"costMicros"`
}

// LLMUsageDay is one day's totals, the day in the caller's time zone.
type LLMUsageDay struct {
	Date string `json:"date"` // YYYY-MM-DD
	LLMUsageTotals
}

// LLMUsageGroup is the totals of one purpose, or of one provider's model.
type LLMUsageGroup struct {
	Key      string `json:"key"`                // purpose, or the model
	Provider string `json:"provider,omitempty"` // set when grouped by model
	LLMUsageTotals
}

// LLMUsageSummary is a budget's model use since Since: the totals, each day
// that had calls (oldest first), and the totals by purpose and by model, most
// expensive first.
type LLMUsageSummary struct {
	Since     time.Time       `json:"since"`
	Totals    LLMUsageTotals  `json:"totals"`
	Days      []LLMUsageDay   `json:"days"`
	ByPurpose []LLMUsageGroup `json:"byPurpose"`
	ByModel   []LLMUsageGroup `json:"byModel"`
}
