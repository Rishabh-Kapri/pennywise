package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// ollamaClient is the backfill's own Ollama at OLLAMA_URL. The API server
// never uses it: there, every model is the budget owner's own.
type ollamaClient struct {
	client      *transport.Client
	callTimeout time.Duration
}

const (
	extractionModel = "gemma4:12b"
	embeddingModel  = "bge-m3"
)

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

type ollamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Format  string         `json:"format"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
}

// extractEmailData reads merchant, amount and account from raw email text.
func (c *ollamaClient) extractEmailData(
	ctx context.Context,
	rawText string,
) (*sharedModel.ExtractedEmailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := transport.Post[ollamaGenerateResponse](ctx, c.client, "/api/generate", nil, ollamaGenerateRequest{
		Model:   extractionModel,
		Prompt:  service.EmailExtractionPrompt + rawText + "\"\nOutput:",
		Format:  "json",
		Options: map[string]any{"temperature": 0.0, "top_p": 1.0},
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "ollama generate", err)
	}
	extracted, err := utils.UnmarshalResponse[sharedModel.ExtractedEmailResponse]([]byte(resp.Response))
	if err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}
	return &extracted, nil
}

// embed returns the vector for text from the given Ollama model.
func (c *ollamaClient) embed(ctx context.Context, model, text string) ([]float64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := transport.Post[ollamaEmbedResponse](ctx, c.client, "/api/embed", nil, ollamaEmbedRequest{
		Model: model,
		Input: text,
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "ollama embed", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, errs.New(errs.CodeInternalError, "ollama embed: no embeddings returned")
	}
	return resp.Embeddings[0], nil
}
