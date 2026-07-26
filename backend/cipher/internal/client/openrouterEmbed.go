package client

import (
	"context"
	"fmt"

	cfg "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	openRouterBaseURL   = "https://openrouter.ai"
	openRouterEmbedPath = "/api/v1/embeddings"
)

// OpenRouterEmbedClient serves embeddings from OpenRouter's OpenAI-compatible
// /api/v1/embeddings endpoint. It exists so the email pipeline can keep
// embedding when a self-hosted Ollama is unreachable — pair it only with a
// model OpenRouter serves under the same weights (e.g. baai/bge-m3 against a
// local bge-m3), so vectors stay comparable to what is already stored.
type OpenRouterEmbedClient struct {
	client *transport.Client
	config cfg.Config
	tracer oteltrace.Tracer
}

type openRouterEmbedReq struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type openRouterEmbedRes struct {
	Model string `json:"model"`
	Data  []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// NewOpenRouterEmbedClient returns nil when no API key is configured, so
// callers can register it unconditionally and let the chain skip it.
func NewOpenRouterEmbedClient(tracer oteltrace.Tracer) *OpenRouterEmbedClient {
	config := cfg.Load()
	if config.OpenRouterAPIKey == "" {
		return nil
	}

	headers := map[string][]string{
		"content-type":  {"application/json"},
		"Authorization": {fmt.Sprintf("Bearer %s", config.OpenRouterAPIKey)},
	}

	return &OpenRouterEmbedClient{
		client: transport.NewClient(
			"openrouter-embed",
			httpclient.NewHttpTransport(openRouterBaseURL),
			transport.WithDefaultHeaders(headers),
			// Never leak internal correlation/auth headers to a third party.
			transport.WithPropagateInternalHeaders(false),
		),
		config: config,
		tracer: tracer,
	}
}

func (c *OpenRouterEmbedClient) Embed(
	ctx context.Context,
	embedModel string,
	text string,
) ([]float64, error) {
	ctx, span := c.tracer.Start(ctx, "embed "+embedModel,
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()
	span.SetAttributes(
		attribute.String("gen_ai.system", "openrouter"),
		attribute.String("gen_ai.request.model", embedModel),
		attribute.String("gen_ai.prompt", text),
	)

	timeout := c.config.LLMCallTimeout
	if timeout <= 0 {
		timeout = cfg.DefaultLLMCallTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := transport.Post[openRouterEmbedRes](ctx, c.client, openRouterEmbedPath, nil, openRouterEmbedReq{
		Model: embedModel,
		Input: text,
	})
	if err != nil {
		err := errs.Wrap(errs.CodeInternalError, "openrouter embed", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if len(res.Data) == 0 || len(res.Data[0].Embedding) == 0 {
		err := errs.New(errs.CodeInternalError, "openrouter embed: no embeddings returned")
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(attribute.Int("gen_ai.usage.output_tokens", len(res.Data[0].Embedding)))
	return res.Data[0].Embedding, nil
}
