package client

import (
	"context"
	"fmt"
	"strings"

	cfg "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

type OllamaClient struct {
	// use abstract Transport client
	client *transport.Client
	config cfg.Config
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

// For OLLAMA local models
type generateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Format  string         `json:"format"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options"`
}
type generateResponse struct {
	Response string `json:"response"`
}

// For OpenAI models
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ResponseFormat struct {
	Type string `json:"type"`
}
type openaiGenerateRequest struct {
	Model          string         `json:"model"`
	Messages       []Message      `json:"messages"`
	ResponseFormat ResponseFormat `json:"response_format"`
}
type openaiGenerateResponse struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
			AudioTokens  int `json:"audio_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens          int `json:"reasoning_tokens"`
			AudioTokens              int `json:"audio_tokens"`
			AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
			RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	ServiceTier       string `json:"service_tier"`
	SystemFingerprint any    `json:"system_fingerprint"`
}

func NewOllamaClient(c *transport.Client) *OllamaClient {
	return &OllamaClient{client: c, config: cfg.Load()}
}

func (c *OllamaClient) Embed(ctx context.Context, model string, text string) ([]float64, error) {
	reqBody := embedRequest{
		Model: model,
		Input: text,
	}
	var headers map[string][]string

	logger.Logger(ctx).Debug("ollama embed", "model", model, "text", text, "client", c.client)
	resp, err := transport.Post[embedResponse](ctx, c.client, "/api/embed", headers, reqBody)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error in ollama embed", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, errs.New(errs.CodeInternalError, "ollama embed: no embeddings returned")
	}

	return resp.Embeddings[0], nil
}

func (c *OllamaClient) Generate(ctx context.Context, model string, prompt string) (string, error) {
	reqBody := generateRequest{
		Model:  model,
		Prompt: prompt,
		Format: "json",
		Stream: false,
		Options: map[string]any{
			"temperature": 0.0, // 0.0 = deterministic
			"top_p":       1.0, // 1.0 = strictly follow the prompt
		},
	}
	var resp generateResponse
	var headers map[string][]string
	// if strings.HasPrefix("ollama") {
	// }
	if strings.HasPrefix(model, "openai") {
		headers = map[string][]string{
			"Authorization": {fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey)},
		}
		openAIModel := strings.ReplaceAll(model, "openai/", "")
		req := openaiGenerateRequest{
			Model: openAIModel,
			Messages: []Message{
				{
					Role:    "user",
					Content: prompt,
				},
			},
			ResponseFormat: ResponseFormat{
				Type: "json_object",
			},
		}
		resp, err := transport.Post[openaiGenerateResponse](ctx, c.client, "https://api.openai.com/v1/chat/completions", headers, req)
		logger.Logger(ctx).Debug("openai generate", "resp", resp, "err", err)
		if err != nil {
			return "", errs.Wrap(errs.CodeInternalError, "error in openai generate", err)
		}
		return resp.Choices[0].Message.Content, nil
	}

	// Local LLM call
	resp, err := transport.Post[generateResponse](ctx, c.client, "/api/generate", headers, reqBody)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "error in ollama generate", err)
	}

	return resp.Response, nil
}
