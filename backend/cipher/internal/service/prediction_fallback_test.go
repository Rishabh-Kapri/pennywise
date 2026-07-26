package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/stretchr/testify/require"
)

// fakeLLM answers with replyText, or fails with err when err is non-nil.
type fakeLLM struct {
	replyText string
	err       error
	// gotModel records the model on the last request it received.
	gotModel string
	calls    int
}

func (f *fakeLLM) Chat(
	ctx context.Context,
	req sharedModel.ChatRequest,
) (*sharedModel.ChatResponse, error) {
	f.calls++
	f.gotModel = req.Model
	if f.err != nil {
		return nil, f.err
	}
	return &sharedModel.ChatResponse{
		Model: req.Model,
		Message: sharedModel.AgentMessage{
			Content: []sharedModel.ContentBlock{{Type: "text", Text: f.replyText}},
		},
	}, nil
}

func (f *fakeLLM) Stream(ctx context.Context, req sharedModel.ChatRequest) <-chan sharedModel.StreamChunk {
	ch := make(chan sharedModel.StreamChunk)
	close(ch)
	return ch
}

// fakeResolver maps provider names to fake clients. Providers absent from the
// map resolve with an error, standing in for a missing API key.
type fakeResolver struct {
	clients   map[string]*fakeLLM
	models    map[string]string
	telemetry otelSDK.TelemetryProvider
}

func (r *fakeResolver) Resolve(provider, model string) (*llm.ObservedLLM, string, error) {
	client, ok := r.clients[provider]
	if !ok {
		return nil, "", errors.New("provider not configured: " + provider)
	}
	if model == "" {
		model = r.models[provider]
	}
	return llm.NewObservedLLM(client, r.telemetry), model, nil
}

func newTestService(t *testing.T, resolver llm.LLMResolver, targets []config.LLMTarget) *predictionService {
	t.Helper()
	return &predictionService{
		llmResolver:     resolver,
		pipelineTargets: targets,
	}
}

func newTestTelemetry(t *testing.T) otelSDK.TelemetryProvider {
	t.Helper()
	// No OTEL_*_EXPORTER env vars in tests, so no exporters are attached.
	tel, err := otelSDK.NewTelemetry(context.Background(), otelSDK.Config{ServiceName: "cipher-test"})
	require.NoError(t, err)
	return tel
}

func simpleReq(model string) sharedModel.ChatRequest {
	return sharedModel.ChatRequest{
		Model: model,
		Messages: []sharedModel.AgentMessage{{
			Role:    sharedModel.RoleUser,
			Content: []sharedModel.ContentBlock{{Type: "text", Text: "hello"}},
		}},
	}
}

// TestChatWithFallbackUsesFirstHealthyProvider: the primary answers, so the
// secondary is never called.
func TestChatWithFallbackUsesFirstHealthyProvider(t *testing.T) {
	primary := &fakeLLM{replyText: `{"ok":true}`}
	secondary := &fakeLLM{replyText: `{"ok":false}`}
	resolver := &fakeResolver{
		clients:   map[string]*fakeLLM{"ollama": primary, "openrouter": secondary},
		models:    map[string]string{"ollama": "gemma4", "openrouter": "gemini"},
		telemetry: newTestTelemetry(t),
	}
	service := newTestService(t, resolver, []config.LLMTarget{
		{Provider: "ollama", Model: "gemma4:12b"},
		{Provider: "openrouter", Model: "google/gemini-2.5-flash"},
	})

	res, err := service.chatWithFallback(context.Background(), "parse:extract", simpleReq)

	require.NoError(t, err)
	require.Equal(t, `{"ok":true}`, res.Message.Content[0].Text)
	require.Equal(t, 1, primary.calls)
	require.Zero(t, secondary.calls, "secondary must not be called when the primary succeeds")
	// The configured model for that target is what gets sent.
	require.Equal(t, "gemma4:12b", primary.gotModel)
}

// TestChatWithFallbackFallsThroughOnFailure is the ollama-is-down case: the
// primary errors and the next provider transparently serves the request.
func TestChatWithFallbackFallsThroughOnFailure(t *testing.T) {
	primary := &fakeLLM{err: errors.New("status code: 530: error code: 1033")}
	secondary := &fakeLLM{replyText: `{"merchant":"Amazon"}`}
	resolver := &fakeResolver{
		clients:   map[string]*fakeLLM{"ollama": primary, "openrouter": secondary},
		models:    map[string]string{"ollama": "gemma4", "openrouter": "gemini"},
		telemetry: newTestTelemetry(t),
	}
	service := newTestService(t, resolver, []config.LLMTarget{
		{Provider: "ollama", Model: "gemma4:12b"},
		{Provider: "openrouter", Model: "google/gemini-2.5-flash"},
	})

	res, err := service.chatWithFallback(context.Background(), "parse:extract", simpleReq)

	require.NoError(t, err, "a healthy fallback provider must keep the pipeline running")
	require.Equal(t, `{"merchant":"Amazon"}`, res.Message.Content[0].Text)
	require.Equal(t, 1, primary.calls)
	require.Equal(t, 1, secondary.calls)
	require.Equal(t, "google/gemini-2.5-flash", secondary.gotModel)
}

// TestChatWithFallbackSkipsUnconfiguredProvider: a provider with no API key is
// skipped rather than treated as a failure of the whole chain.
func TestChatWithFallbackSkipsUnconfiguredProvider(t *testing.T) {
	secondary := &fakeLLM{replyText: `{"ok":true}`}
	resolver := &fakeResolver{
		clients:   map[string]*fakeLLM{"openrouter": secondary},
		models:    map[string]string{"openrouter": "gemini"},
		telemetry: newTestTelemetry(t),
	}
	service := newTestService(t, resolver, []config.LLMTarget{
		{Provider: "anthropic"}, // not configured
		{Provider: "openrouter"},
	})

	res, err := service.chatWithFallback(context.Background(), "predict:llm_fallback", simpleReq)

	require.NoError(t, err)
	require.Equal(t, `{"ok":true}`, res.Message.Content[0].Text)
	// Empty target model falls back to the provider's registry default.
	require.Equal(t, "gemini", secondary.gotModel)
}

// TestChatWithFallbackErrorsWhenAllProvidersFail: the activity still gets an
// error so Temporal retries and, ultimately, the run parks for a manual retry.
func TestChatWithFallbackErrorsWhenAllProvidersFail(t *testing.T) {
	primary := &fakeLLM{err: errors.New("connection refused")}
	secondary := &fakeLLM{err: errors.New("429 rate limited")}
	resolver := &fakeResolver{
		clients:   map[string]*fakeLLM{"ollama": primary, "openrouter": secondary},
		models:    map[string]string{"ollama": "gemma4", "openrouter": "gemini"},
		telemetry: newTestTelemetry(t),
	}
	service := newTestService(t, resolver, []config.LLMTarget{
		{Provider: "ollama"},
		{Provider: "openrouter"},
	})

	_, err := service.chatWithFallback(context.Background(), "parse:extract", simpleReq)

	require.Error(t, err)
	require.Contains(t, err.Error(), "parse:extract")
	require.Equal(t, 1, primary.calls)
	require.Equal(t, 1, secondary.calls)
}

// TestChatWithFallbackErrorsWithoutTargets guards against a misconfigured
// chain silently doing nothing.
func TestChatWithFallbackErrorsWithoutTargets(t *testing.T) {
	service := newTestService(t, &fakeResolver{telemetry: newTestTelemetry(t)}, nil)

	_, err := service.chatWithFallback(context.Background(), "parse:extract", simpleReq)

	require.Error(t, err)
}
