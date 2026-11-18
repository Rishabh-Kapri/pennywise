package repository

import (
	"context"
	"encoding/json"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type CreateAgentConversationParams struct {
	UserID   uuid.UUID
	BudgetID uuid.UUID
	AgentKey string
	Title    *string
	Metadata map[string]any
}

type CreateAgentRunParams struct {
	UserID         uuid.UUID
	BudgetID       uuid.UUID
	AgentKey       string
	ConversationID uuid.UUID
	ModelProvider  *string
	ModelName      *string
	Temperature    *float64
	MaxTokens      *int
	Metadata       map[string]any
}

// StartJobRunParams starts (or restarts, on a retry) the agent run of a
// pipeline job that calls models.
type StartJobRunParams struct {
	JobID    uuid.UUID
	BudgetID uuid.UUID
	AgentKey string // model.AgentKeyPipeline
	Metadata map[string]any
}

type CreateConversationMessageParams struct {
	ConversationID uuid.UUID
	RunID          *uuid.UUID
	Role           model.Role
	Content        json.RawMessage
	Metadata       map[string]any
}

// RecordAgentRunResult is what an agent run leaves behind when it finishes.
// The token, cost, model and turn columns aren't in it: RecordRunResult rolls
// them up from the run's llm_usage rows.
type RecordAgentRunResult struct {
	RunID           uuid.UUID
	BudgetID        uuid.UUID
	StopReason      string
	PromptVersion   string
	ToolInvocations []model.ToolInvocation
}

type AgentRepository interface {
	CreateConversation(
		ctx context.Context,
		params CreateAgentConversationParams,
	) (*model.AgentConversation, error)
	GetConversationForUpdate(
		ctx context.Context,
		id uuid.UUID,
		userID uuid.UUID,
		budgetID uuid.UUID,
		agentKey string,
	) (*model.AgentConversation, error)
	// GetAllConversations lists the user's conversations, newest first, each
	// with the usage of its runs.
	GetAllConversations(ctx context.Context, userID uuid.UUID, budgetID uuid.UUID) ([]model.AgentConversation, error)
	// ConversationUsage totals the model calls of a conversation's runs, in
	// all and per run, from llm_usage (every purpose: turns, title, memory).
	ConversationUsage(ctx context.Context, budgetID, conversationID uuid.UUID) (model.ConversationUsage, error)
	GetAllConversationRuns(ctx context.Context, runID uuid.UUID) ([]model.AgentRun, error)
	GetConversation(
		ctx context.Context,
		id uuid.UUID,
		userID uuid.UUID,
		budgetID uuid.UUID,
	) (*model.AgentConversation, error)
	CreateRun(ctx context.Context, params CreateAgentRunParams) (*model.AgentRun, error)
	// StartJobRun marks a job's agent run RUNNING and returns it. The first
	// attempt creates the run, for the budget's owner, and saves its id on the
	// job (jobs.agent_run_id); a retry reuses that run, so every attempt's
	// model calls add up in one place.
	StartJobRun(ctx context.Context, params StartJobRunParams) (*model.AgentRun, error)
	GetRun(ctx context.Context, userID uuid.UUID, budgetID uuid.UUID, id uuid.UUID) (*model.AgentRun, error)
	UpdateRunStatus(
		ctx context.Context,
		id uuid.UUID,
		budgetID uuid.UUID,
		status model.AgentRunStatus,
		errorMessage *string,
	) (*model.AgentRun, error)
	CreateConversationMessage(
		ctx context.Context,
		params CreateConversationMessageParams,
	) (*model.ConversationMessage, error)
	ListConversationMessages(
		ctx context.Context,
		conversationID uuid.UUID,
		runID *uuid.UUID,
	) ([]model.ConversationMessage, error)
	UpdateConversation(ctx context.Context, conversationID uuid.UUID, data model.AgentConversation) error
	DeleteConversation(
		ctx context.Context,
		conversationID uuid.UUID,
		userID uuid.UUID,
		budgetID uuid.UUID,
	) error
	// currently appends a whole message content object
	// @TODO: support content message part append
	UpdateConversationMessageContent(
		ctx context.Context,
		messageID uuid.UUID,
		data []model.MessagePart,
	) error
	// RecordRunResult saves a finished run's tool invocations and sets its
	// metric columns (tokens, cost, resolved model, turn and tool-call counts
	// rolled up from llm_usage and tool_invocations) in one transaction. A chat
	// run counts only its agent_turn calls; any other agent counts every call. It
	// replaces the run's earlier invocations, so a retry doesn't add them twice.
	RecordRunResult(ctx context.Context, result RecordAgentRunResult) error
	UpdateEntityMetadata(
		ctx context.Context,
		entity string,
		id uuid.UUID,
		data map[string]any,
	) error
}
