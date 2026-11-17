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
