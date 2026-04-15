package client

import (
	"context"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

type OllamaClient struct {
	// use abstract Transport client
	client *transport.Client
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Format string `json:"format"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
}

func NewOllamaClient(c *transport.Client) *OllamaClient {
	return &OllamaClient{client: c}
}

func (c *OllamaClient) Embed(ctx context.Context, model string, text string) ([]float64, error) {
	reqBody := embedRequest{
		Model: model,
		Input: text,
	}

	logger.Logger(ctx).Debug("ollama embed", "model", model, "text", text, "client", c.client)
	resp, err := transport.Post[embedResponse](ctx, c.client, "/api/embed", reqBody)
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
	}

	resp, err := transport.Post[generateResponse](ctx, c.client, "/api/generate", reqBody)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "error in ollama generate", err)
	}

	return resp.Response, nil
}
