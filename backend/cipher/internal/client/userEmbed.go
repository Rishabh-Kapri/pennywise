package client

import (
	"context"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

// EmbeddingDimensions is the size of every stored vector (vector(1024) columns).
const EmbeddingDimensions = 1024

const (
	openRouterBaseURL   = "https://openrouter.ai"
	openRouterEmbedPath = "/api/v1/embeddings"
	ollamaEmbedPath     = "/api/embed"
)

type userEmbedder struct {
	tc       *transport.Client
	provider model.LLMProviderName
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float64 `json:"embeddings"` // Ollama
	Data       []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"` // OpenRouter
}

// NewUserEmbedder embeds with a user's own key. httpClient must be the
// SSRF-safe client: an Ollama URL is user input.
func NewUserEmbedder(
	provider model.LLMProviderName,
	baseURL, apiKey string,
	httpClient *http.Client,
) (Embedder, error) {
	var engine transport.Transport
	switch provider {
	case model.LLMProviderOllama:
		if baseURL == "" {
			return nil, errs.New(errs.CodeAIProviderNotConfigured, "no ollama url found")
		}
		engine = httpclient.NewHttpTransportWithClient(baseURL, httpClient)
	case model.LLMProviderOpenRouter:
		engine = httpclient.NewHttpTransport(openRouterBaseURL)
	default:
		return nil, errs.New(errs.CodeAIProviderNotConfigured, "%s keys can't serve embeddings", provider)
	}

	opts := []transport.Option{transport.WithPropagateInternalHeaders(false)}
	if apiKey != "" {
		opts = append(opts, transport.WithDefaultHeaders(map[string][]string{"Authorization": {"Bearer " + apiKey}}))
	}
	return &userEmbedder{tc: transport.NewClient("user-embed", engine, opts...), provider: provider}, nil
}

// Embed implements [Embedder]. A vector of the wrong size is an error here,
// because the database would refuse it later.
func (e *userEmbedder) Embed(ctx context.Context, slug, text string) ([]float64, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultLLMCallTimeout)
	defer cancel()

	path := openRouterEmbedPath
	if e.provider == model.LLMProviderOllama {
		path = ollamaEmbedPath
	}
	res, err := transport.Post[embedResponse](ctx, e.tc, path, nil, embedRequest{Model: slug, Input: text})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "embed with "+slug, err)
	}

	var vector []float64
	if len(res.Embeddings) > 0 {
		vector = res.Embeddings[0]
	} else if len(res.Data) > 0 {
		vector = res.Data[0].Embedding
	}
	if len(vector) != EmbeddingDimensions {
		return nil, errs.New(
			errs.CodeInternalError, "%s returned %d dimensions; Pennywise needs %d",
			slug, len(vector), EmbeddingDimensions,
		)
	}
	return vector, nil
}
