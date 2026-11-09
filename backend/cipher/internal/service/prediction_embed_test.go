package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// fakeEmbedder returns vector, or fails with err when err is non-nil.
type fakeEmbedder struct {
	vector   []float64
	err      error
	gotModel string
}

func (f *fakeEmbedder) Embed(_ context.Context, model string, _ string) ([]float64, error) {
	f.gotModel = model
	return f.vector, f.err
}

type fakeEmbedModels struct {
	repository.UserLLMModelRepository
	rows    []sharedModel.UserLLMModel
	gotKind []sharedModel.LLMModelKind
}

func (f *fakeEmbedModels) ListUsableForBudget(
	_ context.Context, _ uuid.UUID, kinds ...sharedModel.LLMModelKind,
) ([]sharedModel.UserLLMModel, error) {
	f.gotKind = kinds
	return f.rows, nil
}

type fakeEmbedResolver struct{ embedder client.Embedder }

func (f fakeEmbedResolver) ResolveEmbedder(sharedModel.UserLLMModel) (client.Embedder, error) {
	return f.embedder, nil
}

func embedContext() context.Context {
	return utils.WithBudgetID(context.Background(), uuid.New())
}

func TestEmbedUsesTheDefaultModel(t *testing.T) {
	embedder := &fakeEmbedder{vector: []float64{1}}
	models := &fakeEmbedModels{rows: []sharedModel.UserLLMModel{
		{Slug: "bge-m3", Provider: sharedModel.LLMProviderOllama, IsDefault: true},
		{Slug: "other", Provider: sharedModel.LLMProviderOpenRouter},
	}}
	svc := &predictionService{llmModels: models, embedders: fakeEmbedResolver{embedder}}

	vector, used, err := svc.embed(embedContext(), "text")
	require.NoError(t, err)
	require.Equal(t, []float64{1}, vector)
	require.Equal(t, "bge-m3", used.Slug)
	require.Equal(t, "bge-m3", embedder.gotModel)
	require.Equal(t, []sharedModel.LLMModelKind{sharedModel.LLMModelEmbedding}, models.gotKind)
}

func TestEmbedErrorsWithoutAModel(t *testing.T) {
	svc := &predictionService{llmModels: &fakeEmbedModels{}, embedders: fakeEmbedResolver{}}
	_, _, err := svc.embed(embedContext(), "text")
	require.True(t, errs.HasCode(err, errs.CodeAIProviderNotConfigured), "%v", err)
}

// A disabled default mustn't be replaced: vectors from another model wouldn't
// be comparable to what's stored.
func TestEmbedDoesNotSwitchModelsWhenTheDefaultIsDisabled(t *testing.T) {
	embedder := &fakeEmbedder{vector: []float64{1}}
	models := &fakeEmbedModels{rows: []sharedModel.UserLLMModel{{Slug: "other"}}}
	svc := &predictionService{llmModels: models, embedders: fakeEmbedResolver{embedder}}

	_, _, err := svc.embed(embedContext(), "text")
	require.True(t, errs.HasCode(err, errs.CodeAIProviderNotConfigured), "%v", err)
	require.Empty(t, embedder.gotModel)
}

func TestEmbedDoesNotTryAnotherModelOnFailure(t *testing.T) {
	embedder := &fakeEmbedder{err: errors.New("down")}
	models := &fakeEmbedModels{rows: []sharedModel.UserLLMModel{{Slug: "bge-m3", IsDefault: true}, {Slug: "other"}}}
	svc := &predictionService{llmModels: models, embedders: fakeEmbedResolver{embedder}}

	_, _, err := svc.embed(embedContext(), "text")
	require.Error(t, err)
	require.Equal(t, "bge-m3", embedder.gotModel)
}
