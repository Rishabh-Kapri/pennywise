package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/stretchr/testify/require"
)

// fakeEmbedder returns vector, or fails with err when err is non-nil.
type fakeEmbedder struct {
	vector   []float64
	err      error
	gotModel string
	calls    int
}

func (f *fakeEmbedder) Embed(ctx context.Context, model string, text string) ([]float64, error) {
	f.calls++
	f.gotModel = model
	if f.err != nil {
		return nil, f.err
	}
	return f.vector, nil
}

func newEmbedService(
	embedders map[string]client.Embedder,
	targets []config.LLMTarget,
) *predictionService {
	return &predictionService{embedders: embedders, embeddingTargets: targets}
}

// TestEmbedWithFallbackUsesFirstHealthyProvider: the local embedder answers, so
// no remote call is made.
func TestEmbedWithFallbackUsesFirstHealthyProvider(t *testing.T) {
	local := &fakeEmbedder{vector: []float64{0.1, 0.2}}
	remote := &fakeEmbedder{vector: []float64{0.3, 0.4}}
	service := newEmbedService(
		map[string]client.Embedder{"ollama": local, "openrouter": remote},
		[]config.LLMTarget{
			{Provider: "ollama", Model: "bge-m3"},
			{Provider: "openrouter", Model: "baai/bge-m3"},
		},
	)

	vector, target, err := service.embedWithFallback(context.Background(), "debit AMAZON")

	require.NoError(t, err)
	require.Equal(t, []float64{0.1, 0.2}, vector)
	require.Equal(t, "ollama", target.Provider)
	require.Equal(t, "bge-m3", target.Model)
	require.Zero(t, remote.calls, "remote must not be called when local succeeds")
}

// TestEmbedWithFallbackFallsThroughOnFailure is the ollama-is-down case.
func TestEmbedWithFallbackFallsThroughOnFailure(t *testing.T) {
	local := &fakeEmbedder{err: errors.New("status code: 530")}
	remote := &fakeEmbedder{vector: []float64{0.3, 0.4}}
	service := newEmbedService(
		map[string]client.Embedder{"ollama": local, "openrouter": remote},
		[]config.LLMTarget{
			{Provider: "ollama", Model: "bge-m3"},
			{Provider: "openrouter", Model: "baai/bge-m3"},
		},
	)

	vector, target, err := service.embedWithFallback(context.Background(), "debit AMAZON")

	require.NoError(t, err)
	require.Equal(t, []float64{0.3, 0.4}, vector)
	// The returned target tells the caller which backend produced the vector.
	require.Equal(t, "openrouter", target.Provider)
	require.Equal(t, "baai/bge-m3", target.Model)
	require.Equal(t, 1, local.calls)
	require.Equal(t, 1, remote.calls)
}

// TestEmbedWithFallbackSkipsUnregisteredProvider: a provider named in the chain
// but never registered (no API key) is skipped, not fatal.
func TestEmbedWithFallbackSkipsUnregisteredProvider(t *testing.T) {
	local := &fakeEmbedder{vector: []float64{0.1}}
	service := newEmbedService(
		map[string]client.Embedder{"ollama": local},
		[]config.LLMTarget{
			{Provider: "openrouter", Model: "baai/bge-m3"},
			{Provider: "ollama", Model: "bge-m3"},
		},
	)

	vector, target, err := service.embedWithFallback(context.Background(), "debit AMAZON")

	require.NoError(t, err)
	require.Equal(t, []float64{0.1}, vector)
	require.Equal(t, "ollama", target.Provider)
}

// TestEmbedWithFallbackDefaultsModel: an entry with no model uses the pipeline's
// embedding model rather than sending an empty one.
func TestEmbedWithFallbackDefaultsModel(t *testing.T) {
	local := &fakeEmbedder{vector: []float64{0.1}}
	service := newEmbedService(
		map[string]client.Embedder{"ollama": local},
		[]config.LLMTarget{{Provider: "ollama"}},
	)

	_, target, err := service.embedWithFallback(context.Background(), "debit AMAZON")

	require.NoError(t, err)
	require.Equal(t, EmbeddingModel, local.gotModel)
	require.Equal(t, EmbeddingModel, target.Model)
}

// TestEmbedWithFallbackErrorsWhenAllProvidersFail: callers decide what a total
// failure means — semantic search treats it as "no match" and moves on.
func TestEmbedWithFallbackErrorsWhenAllProvidersFail(t *testing.T) {
	local := &fakeEmbedder{err: errors.New("connection refused")}
	remote := &fakeEmbedder{err: errors.New("429 rate limited")}
	service := newEmbedService(
		map[string]client.Embedder{"ollama": local, "openrouter": remote},
		[]config.LLMTarget{{Provider: "ollama"}, {Provider: "openrouter"}},
	)

	_, _, err := service.embedWithFallback(context.Background(), "debit AMAZON")

	require.Error(t, err)
	require.Equal(t, 1, local.calls)
	require.Equal(t, 1, remote.calls)
}

// TestEmbedWithFallbackErrorsWithoutTargets guards a misconfigured chain.
func TestEmbedWithFallbackErrorsWithoutTargets(t *testing.T) {
	service := newEmbedService(map[string]client.Embedder{}, nil)

	_, _, err := service.embedWithFallback(context.Background(), "debit AMAZON")

	require.Error(t, err)
}

// TestNewPredictionServiceDropsNilEmbedder: an unconfigured constructor returns
// a typed nil pointer, which must not end up in the chain as a live backend.
func TestNewPredictionServiceDropsNilEmbedder(t *testing.T) {
	var unconfigured *client.OpenRouterEmbedClient // nil, as with no API key

	service := NewPredictionService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		map[string]client.Embedder{"openrouter": unconfigured},
	).(*predictionService)

	_, registered := service.embedders["openrouter"]
	require.False(t, registered, "a typed-nil embedder must not be registered")
}
