package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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

	body, err := c.post(ctx, "/api/embed", reqBody)
	if err != nil {
		return nil, fmt.Errorf("Embed: %w", err)
	}

	var resp embedResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("Embed unmarshal: %w", err)
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

	body, err := c.post(ctx, "/api/generate", reqBody)
	if err != nil {
		return "", fmt.Errorf("Generate: %w", err)
	}

	var resp generateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("Generate unmarshal: %w", err)
	}

	return resp.Response, nil
}

func (c *OllamaClient) post(ctx context.Context, path string, data any) ([]byte, error) {
	reqBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("ollama request failed", "url", url, "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("request to %s failed with status %d", url, resp.StatusCode)
	}

	return body, nil
}
