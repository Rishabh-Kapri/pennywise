package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type StopReason string

const (
	StopReasonEndTurn   StopReason = "end_turn"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonMaxTokens StopReason = "max_tokens"
	StopReasonError     StopReason = "error"
)

type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type ContentBlock struct {
	Type string
	Text string
}

type AgentMessage struct {
	Sequence   int
	Role       Role
	Content    []ContentBlock
	ToolCalls  []ToolCall
	ToolResult *ToolResult
	Name       string
}

type ChatRequest struct {
	Provider    string
	Model       string
	Messages    []AgentMessage
	Tools       []ToolDefiniton
	ToolChoice  []ToolChoice
	Temperature float32
	MaxTokens   int
	Metadata    map[string]string
	Stream      bool
	Format      string // "json" forces JSON output (Ollama only for now)
}

type ChatResponse struct {
	ID          string
	Model       string
	Message     AgentMessage
	Usage       Usage
	StopReason  StopReason
	RawProvider any // this is the raw message from the llm provider
}

type ToolDefiniton struct {
	Name        string
	Description string
	InputSchema ToolSchema
}

type ToolSchema struct {
	Type                 string                `json:"type"`
	Items                *ToolSchema           `json:"items,omitempty"`
	Enum                 *[]any                `json:"enum,omitempty"`
	Properties           map[string]ToolSchema `json:"properties,omitempty"`
	Required             []string              `json:"required,omitempty"`
	Description          string                `json:"description,omitempty"`
	AdditionalProperties bool                  `json:"additionalProperties,omitempty"`
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ToolResult struct {
	ToolCallId string
	Name       string
	Content    []ContentBlock
	IsError    bool
}

type ToolResultNormalized struct {
	DisplayName string
	Summary     string
	Result      json.RawMessage
}

type ToolChoice struct {
	Type string
	Name string
}

const (
	ToolChoiceAuto     = "auto"
	ToolChoiceNone     = "none"
	ToolChoiceRequired = "required"
	ToolChoiceSpecific = "specific"
)

type ChunkEvent string

const (
	ChunkEventStarted       ChunkEvent = "started"
	ChunkEventText          ChunkEvent = "text"
	ChunkEventMessage       ChunkEvent = "message"
	ChunkEventToolCallStart ChunkEvent = "tool_call_start"
	ChunkEventToolCall      ChunkEvent = "tool_call"
	ChunkEventToolCallDelta ChunkEvent = "tool_call_delta"
	ChunkEventToolResult    ChunkEvent = "tool_result"
	ChunkEventCompleted     ChunkEvent = "completed"
	ChunkEventError         ChunkEvent = "error"
)

type StreamChunk struct {
	Type ChunkEvent
	Text string // Text Content (for ChunkEvent)

	// Tool call fields (for ChunkEventToolCallDelta, ChunkEventToolCall, ChunkEventToolCallStart)
	ToolCallID    string
	ToolName      string
	ToolArgsDelta string

	OutputIndex int

	Usage      Usage
	StopReason StopReason
}

// Accumulated result of a single llm call
type StepResult struct {
	Text       string
	ToolCalls  []ToolCall
	Usage      Usage
	MaxTokens  int
	Err        error
	StopReason StopReason
}

type PubsubEvent struct {
	Event string
	Data  any
}

// IntentResult is the parsed output of the intent classification LLM call.
// The cloud LLM receives only the user query, today's date, and category group
// names. No entity IDs, payee names, account names, or balances are included.
type IntentResult struct {
	Intent         string     `json:"intent"`
	DateRange      *DateRange `json:"dateRange"`
	CategoryGroups []string   `json:"categoryGroups"`
	PayeeTerms     []string   `json:"payeeTerms"`
	Confidence     float64    `json:"confidence"`
}

// DateRange is a simple from/to date pair in YYYY-MM-DD format.
type DateRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type AgentEvent string

const (
	AgentEventChatSubscribe AgentEvent = "pennywise::agent::chat::subscribe"
	AgentEventChatStream    AgentEvent = "pennywise::agent::chat::stream"
)

type AgentRunStatus string

const (
	AgentRunStatusQueued    AgentRunStatus = "QUEUED"
	AgentRunStatusRunning   AgentRunStatus = "RUNNING"
	AgentRunStatusCompleted AgentRunStatus = "COMPLETED"
	AgentRunStatusFailed    AgentRunStatus = "FAILED"
	AgentRunStatusCancelled AgentRunStatus = "CANCELLED"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeToolCall MessageType = "tool_call"
)

type AgentConversation struct {
	ID        uuid.UUID      `json:"id"`
	AgentKey  string         `json:"agentKey"`
	UserID    uuid.UUID      `json:"userId"`
	BudgetID  uuid.UUID      `json:"budgetId"`
	Title     *string        `json:"title,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type MessagePart struct {
	Type        MessageType     `json:"type"`
	Content     *string         `json:"content,omitempty"`
	ID          *string         `json:"id,omitempty"`
	Name        *string         `json:"name,omitempty"`
	DisplayName *string         `json:"displayName,omitempty"` // display name of the tool
	Summary     *string         `json:"summary,omitempty"`     // summary of the tool
	Result      json.RawMessage `json:"result,omitempty"`      // normalized result of the tool
}

type ConversationMessage struct {
	ID             uuid.UUID       `json:"id"`
	ConversationID uuid.UUID       `json:"conversationId"`
	RunID          *uuid.UUID      `json:"runId,omitempty"`
	Role           Role            `json:"role"`
	Sequence       int             `json:"sequence"`
	Content        json.RawMessage `json:"content"` // content parts of the convo
	Metadata       map[string]any  `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type AgentRunCreateRequest struct {
	Message              string                `json:"message"`
	PrevMessages         []ConversationMessage `json:"prevMessages,omitempty"`
	PrevRuns             []AgentRun            `json:"prevRuns,omitempty"`
	BudgetID             *uuid.UUID            `json:"budgetId,omitempty"`
	RunID                *uuid.UUID            `json:"runId,omitempty"`
	AgentKey             *string               `json:"agentKey,omitempty"`
	ConversationID       *uuid.UUID            `json:"conversationId,omitempty"`
	MessageID            *uuid.UUID            `json:"messageId,omitempty"`
	Title                *string               `json:"title,omitempty"`
	ModelProvider        *string               `json:"modelProvider,omitempty"`
	ModelName            *string               `json:"modelName,omitempty"`
	Temperature          *float64              `json:"temperature,omitempty"`
	MaxTokens            *int                  `json:"maxTokens,omitempty"`
	Stream               bool                  `json:"stream,omitempty"`
	ConversationMetadata map[string]any        `json:"conversationMetadata,omitempty"`
	MessageMetadata      map[string]any        `json:"messageMetadata,omitempty"`
	Metadata             map[string]any        `json:"metadata,omitempty"`
}

type AgentRun struct {
	ID             uuid.UUID             `json:"id"`
	AgentKey       string                `json:"agentKey"`
	UserID         *uuid.UUID            `json:"userId,omitempty"`
	BudgetID       *uuid.UUID            `json:"budgetId,omitempty"`
	ConversationID *uuid.UUID            `json:"conversationId,omitempty"`
	Status         AgentRunStatus        `json:"status"`
	ModelProvider  *string               `json:"modelProvider,omitempty"`
	ModelName      *string               `json:"modelName,omitempty"`
	Temperature    *float64              `json:"temperature,omitempty"`
	MaxTokens      *int                  `json:"maxTokens,omitempty"`
	UserMessage    string                `json:"userMessage,omitempty"`
	FinalMessage   *string               `json:"finalMessage,omitempty"`
	Error          *string               `json:"error,omitempty"`
	TraceID        *string               `json:"traceId,omitempty"`
	StartedAt      *time.Time            `json:"startedAt,omitempty"`
	CompletedAt    *time.Time            `json:"completedAt,omitempty"`
	CreatedAt      *time.Time            `json:"createdAt,omitempty"`
	UpdatedAt      *time.Time            `json:"updatedAt,omitempty"`
	Metadata       map[string]any        `json:"metadata,omitempty"`
	Conversation   *AgentConversation    `json:"conversation,omitempty"`
	Messages       []ConversationMessage `json:"messages,omitempty"`
}

type AgentWorkingMemory struct {
	ID        uuid.UUID       `json:"id"`
	BudgetID  uuid.UUID       `json:"budgetId"`
	Document  json.RawMessage `json:"document"`
	CreatedAt *time.Time      `json:"created_at,omitempty"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"`
}

type ObservationPriority string

const (
	ObservationPriorityHigh      ObservationPriority = "high"
	ObservationPriorityMedium    ObservationPriority = "medium"
	ObservationPriorityLow       ObservationPriority = "low"
	ObservationPriorityCompleted ObservationPriority = "completed"
)

type AgentObservations struct {
	Date                string              `json:"date"`
	Time                string              `json:"time"`
	Priority            ObservationPriority `json:"priority"`
	Text                string              `json:"text"`
	SupportingDetails   []string            `json:"supportingDetails"`
	ReferencedTimeRange string              `json:"referencedTimeRange"`
}

type AgentObservationalMemory struct {
	ID                uuid.UUID           `json:"id"`
	BudgetID          uuid.UUID           `json:"budgetId"`
	UserID            uuid.UUID           `json:"userId"`
	ConversationID    uuid.UUID           `json:"conversationId"`
	SequenceStart     int                 `json:"sequenceStart"`
	SequenceEnd       int                 `json:"sequenceEnd"`
	Observations      []AgentObservations `json:"observations"`
	CurrentTask       *string             `json:"currentTask"`
	SuggestedResponse *string             `json:"suggestedResponse"`
	CreatedAt         *time.Time          `json:"createdAt,omitempty"`
	UpdatedAt         *time.Time          `json:"updatedAt,omitempty"`
	DeletedAt         *time.Time          `json:"deletedAt,omitempty"`
}
