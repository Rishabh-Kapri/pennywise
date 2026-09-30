package providers

import (
	"context"
	"net/url"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

const lumoPremiumQuotaMessage = "you have reached your daily quota for premium models"

type lumoClient struct {
	responses *openAIClient
}

func shouldUseLumoLite(model, message string) bool {
	return model != "lumo-lite" && strings.Contains(strings.ToLower(message), lumoPremiumQuotaMessage)
}

func lumoChunkHasOutput(chunk sharedModel.StreamChunk) bool {
	switch chunk.Type {
	case sharedModel.ChunkEventStarted:
		return false
	case sharedModel.ChunkEventText, sharedModel.ChunkEventReasoning:
		return chunk.Text != ""
	default:
		return true
	}
}

func (c *lumoClient) Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error) {
	res, err := c.responses.Chat(ctx, req)
	if err == nil || ctx.Err() != nil || !shouldUseLumoLite(req.Model, err.Error()) {
		return res, err
	}
	logger.Logger(ctx).Warn("Lumo premium quota reached; retrying with lumo-lite")
	req.Model = "lumo-lite"
	return c.responses.Chat(ctx, req)
}

func (c *lumoClient) Stream(ctx context.Context, req sharedModel.ChatRequest) <-chan sharedModel.StreamChunk {
	out := make(chan sharedModel.StreamChunk)
	go func() {
		defer close(out)
		upstream := c.responses.Stream(ctx, req)
		var quotaError *sharedModel.StreamChunk
		hasOutput := false

		for chunk := range upstream {
			if chunk.Type == sharedModel.ChunkEventError && !hasOutput && shouldUseLumoLite(req.Model, chunk.Text) {
				quotaError = &chunk
				continue
			}
			if quotaError != nil {
				if !lumoChunkHasOutput(chunk) {
					continue
				}
				select {
				case out <- *quotaError:
				case <-ctx.Done():
					return
				}
				quotaError = nil
			}
			if lumoChunkHasOutput(chunk) {
				hasOutput = true
			}
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
		}
		if quotaError == nil || ctx.Err() != nil {
			return
		}
		logger.Logger(ctx).Warn("Lumo premium quota reached; retrying stream with lumo-lite")
		req.Model = "lumo-lite"

		for chunk := range c.responses.Stream(ctx, req) {
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// NewLumoClient connects to lumo-tamer using the shared Responses API adapter.
func NewLumoClient() (llm.LLM, error) {
	cfg := config.Load()
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.LumoBaseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return nil, errs.New(
			errs.CodeInternalError,
			"LUMO_BASE_URL must be an absolute HTTP(S) URL without query or fragment",
		)
	}
	// Accept server roots and conventional OpenAI-compatible /v1 base URLs.
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return &lumoClient{responses: newResponsesClient("lumo", baseURL, cfg.LumoAPIKey)}, nil
}
