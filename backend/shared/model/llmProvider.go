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

// UserLLMProvider is a user_llm_providers row as the API returns it. The key
// itself never leaves the server; HasAPIKey and APIKeyHint describe it.
type UserLLMProvider struct {
	ID             uuid.UUID       `json:"id"`
	UserID         uuid.UUID       `json:"userId"`
	Provider       LLMProviderName `json:"provider"`
	HasAPIKey      bool            `json:"hasApiKey"`
	APIKeyHint     string          `json:"apiKeyHint,omitempty"`
	BaseURL        string          `json:"baseUrl,omitempty"`
	Model          string          `json:"model"`
	Priority       int             `json:"priority"`
	Enabled        bool            `json:"enabled"`
	LastVerifiedAt *time.Time      `json:"lastVerifiedAt,omitempty"`
	LastError      string          `json:"lastError,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	// APIKeyEncrypted is only filled for cipher, which decrypts it.
	APIKeyEncrypted []byte `json:"-"`
}

// UpsertUserLLMProviderRequest is PUT /api/ai-providers/:provider. An omitted
// APIKey keeps the saved one, so the UI can edit the model without re-entering
// the key; an empty string clears it.
type UpsertUserLLMProviderRequest struct {
	APIKey   *string `json:"apiKey"`
	BaseURL  string  `json:"baseUrl"`
	Model    string  `json:"model"`
	Priority *int    `json:"priority"`
	Enabled  *bool   `json:"enabled"`
}

// TestUserLLMProviderResponse is the result of a one-token test call.
type TestUserLLMProviderResponse struct {
	OK        bool   `json:"ok"`
	Model     string `json:"model,omitempty"`
	Error     string `json:"error,omitempty"`
	LatencyMs int64  `json:"latencyMs"`
}
