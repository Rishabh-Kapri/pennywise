package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
)

type OllamaClient struct {
	baseURL string
	client  *http.Client
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

func NewOllamaClient(baseURL string) *OllamaClient {
	return &OllamaClient{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (c *OllamaClient) Embed(ctx context.Context, model string, text string) ([]float64, error) {
	reqBody := embedRequest{
		Model: model,
		Input: text,
	}

	resp, err := httpclient.Post[embedResponse](ctx, c.baseURL+"/api/embed", reqBody)
	if err != nil {
		return nil, fmt.Errorf("Embed: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("Embed: no embeddings returned")
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

	resp, err := httpclient.Post[generateResponse](ctx, c.baseURL+"/api/generate", reqBody)
	if err != nil {
		return "", fmt.Errorf("Generate: %w", err)
	}

	return resp.Response, nil
}
