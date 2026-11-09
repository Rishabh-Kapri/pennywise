package model

import (
	"time"

	"github.com/google/uuid"
)

// LLMProviderName is the llm_provider enum: the providers a user can bring.
type LLMProviderName string

const (
	LLMProviderOpenAI     LLMProviderName = "openai"
	LLMProviderAnthropic  LLMProviderName = "anthropic"
	LLMProviderOpenRouter LLMProviderName = "openrouter"
	LLMProviderOllama     LLMProviderName = "ollama"
)

// Valid reports whether p is a provider users may configure.
func (p LLMProviderName) Valid() bool {
	switch p {
	case LLMProviderOpenAI, LLMProviderAnthropic, LLMProviderOpenRouter, LLMProviderOllama:
		return true
	}
	return false
}

// UserLLMProvider is a saved key: a user_llm_providers row as the API returns
// it. The key itself never leaves the server; HasAPIKey and APIKeyHint
// describe it. Models used with the key are UserLLMModel rows.
type UserLLMProvider struct {
	ID         uuid.UUID       `json:"id"`
	UserID     uuid.UUID       `json:"userId"`
	Provider   LLMProviderName `json:"provider"`
	HasAPIKey  bool            `json:"hasApiKey"`
	APIKeyHint string          `json:"apiKeyHint,omitempty"`
	BaseURL    string          `json:"baseUrl,omitempty"`
	Enabled    bool            `json:"enabled"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	// APIKeyEncrypted is only filled for cipher, which decrypts it.
	APIKeyEncrypted []byte `json:"-"`
}

// UpsertUserLLMProviderRequest is PUT /api/ai-providers/:provider. An omitted
// APIKey keeps the saved one; an empty string clears it (Ollama only).
type UpsertUserLLMProviderRequest struct {
	APIKey  *string `json:"apiKey"`
	BaseURL string  `json:"baseUrl"`
	Enabled *bool   `json:"enabled"`
}

// LLMModelKind is the llm_model_kind enum.
type LLMModelKind string

const (
	// LLMModelChat is a conversational LLM: extraction, prediction, the assistant.
	LLMModelChat LLMModelKind = "chat"
	// LLMModelDecision is a small classification model called through the
	// decisions API at a user-given endpoint.
	LLMModelDecision LLMModelKind = "decision"
	// LLMModelEmbedding turns text into the vectors semantic search compares.
	// One default per user; every stored vector must come from the same model.
	LLMModelEmbedding LLMModelKind = "embedding"
)

func (k LLMModelKind) Valid() bool {
	return k == LLMModelChat || k == LLMModelDecision || k == LLMModelEmbedding
}

// ServesEmbeddings reports whether Pennywise can embed with this provider's keys.
func (p LLMProviderName) ServesEmbeddings() bool {
	return p == LLMProviderOllama || p == LLMProviderOpenRouter
}

// UserLLMModel is a model the user runs with one of their keys
// (user_llm_models).
type UserLLMModel struct {
	ID         uuid.UUID       `json:"id"`
	UserID     uuid.UUID       `json:"userId"`
	ProviderID uuid.UUID       `json:"providerId"`
	Provider   LLMProviderName `json:"provider"`
	Kind       LLMModelKind    `json:"kind"`
	Slug       string          `json:"slug"`
	// Endpoint is the decisions API to call. Decision models only.
	Endpoint  string `json:"endpoint,omitempty"`
	IsDefault bool   `json:"isDefault"`
	// Enabled: a disabled model is kept but never run.
	Enabled bool `json:"enabled"`
	// Priority orders the models of a kind, after the default. Lower runs first.
	Priority       int        `json:"priority"`
	LastVerifiedAt *time.Time `json:"lastVerifiedAt,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`

	// The key's settings, only filled for cipher, which calls the model.
	APIKeyEncrypted []byte    `json:"-"`
	BaseURL         string    `json:"-"`
	KeyUpdatedAt    time.Time `json:"-"`
}

// CreateUserLLMModelRequest is POST /api/ai-models. The model is tested before
// it's saved. The first model of a kind becomes its default.
type CreateUserLLMModelRequest struct {
	Provider    LLMProviderName `json:"provider"`
	Kind        LLMModelKind    `json:"kind"`
	Slug        string          `json:"slug"`
	Endpoint    string          `json:"endpoint"`
	MakeDefault bool            `json:"makeDefault"`
}

// UpdateUserLLMModelRequest is PUT /api/ai-models/:id. The model is tested
// again before the change is saved.
type UpdateUserLLMModelRequest struct {
	Slug     string `json:"slug"`
	Endpoint string `json:"endpoint"`
}

// SetUserLLMModelEnabledRequest is PUT /api/ai-models/:id/enabled.
type SetUserLLMModelEnabledRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// ReorderUserLLMModelsRequest is PUT /api/ai-models/order: every model of the
// kind, in the order they should run.
type ReorderUserLLMModelsRequest struct {
	Kind LLMModelKind `json:"kind"`
	IDs  []uuid.UUID  `json:"ids"`
}

// TestUserLLMModelRequest is what cipher tests: a saved model (ModelID set,
// and the result is recorded on it) or one about to be saved.
type TestUserLLMModelRequest struct {
	ModelID  *uuid.UUID      `json:"modelId,omitempty"`
	Provider LLMProviderName `json:"provider"`
	Kind     LLMModelKind    `json:"kind"`
	Slug     string          `json:"slug"`
	Endpoint string          `json:"endpoint,omitempty"`
}

// TestUserLLMModelResponse is the result of a tiny test call.
type TestUserLLMModelResponse struct {
	OK        bool   `json:"ok"`
	Model     string `json:"model,omitempty"`
	Error     string `json:"error,omitempty"`
	LatencyMs int64  `json:"latencyMs"`
}
