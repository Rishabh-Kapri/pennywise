package model

// ── Public request/response types ───────────────────────────────

// PromptReq is the unified input for all LLM calls.
// For OpenAI: if UserPrompt is set, Prompt becomes the system message
// and UserPrompt the user message. Otherwise Prompt is sent as a single user message.
type PromptReq struct {
	Model      string `json:"model"`
	Prompt     string `json:"prompt"`
	UserPrompt string `json:"userPrompt"`
}

// ExtractedEmail is the structured output from Phase 1 LLM extraction.
type ExtractedEmail struct {
	Merchant    string  `json:"merchant"`
	Amount      float64 `json:"amount"`
	AccountCard string  `json:"account_card"`
}

// LLMPrediction is the structured output from Phase 4 LLM classification.
type LLMPrediction struct {
	MerchantName string `json:"merchantName"`
	SuggestedTag string `json:"suggestedTag"`
	Confidence   int32  `json:"confidence"`
	Reasoning    string `json:"reasoning"`
}

// ── Ollama-local types ──────────────────────────────────────────

type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

type OllamaRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Format  string         `json:"format"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options"`
}

type OllamaResponse struct {
	Response        string `json:"response"`
	PromptEvalCount int    `json:"prompt_eval_count"` // Tokens in the prompt
	EvalCount       int    `json:"eval_count"`        // Tokens generated
}
