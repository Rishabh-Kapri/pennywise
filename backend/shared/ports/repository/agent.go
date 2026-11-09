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

type CreateConversationMessageParams struct {
	ConversationID uuid.UUID
	RunID          *uuid.UUID
	Role           model.Role
	Content        json.RawMessage
	Metadata       map[string]any
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
	GetAllConversations(ctx context.Context, userID uuid.UUID, budgetID uuid.UUID) ([]model.AgentConversation, error)
	GetAllConversationRuns(ctx context.Context, runID uuid.UUID) ([]model.AgentRun, error)
	GetConversation(
		ctx context.Context,
		id uuid.UUID,
		userID uuid.UUID,
		budgetID uuid.UUID,
	) (*model.AgentConversation, error)
	CreateRun(ctx context.Context, params CreateAgentRunParams) (*model.AgentRun, error)
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
	UpdateEntityMetadata(
		ctx context.Context,
		entity string,
		id uuid.UUID,
		data map[string]any,
	) error
}
