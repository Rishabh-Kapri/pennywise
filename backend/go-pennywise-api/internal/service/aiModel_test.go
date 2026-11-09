package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
)

type fakeLLMModelRepo struct {
	repository.UserLLMModelRepository
	created *repository.UserLLMModelCreate
	updated string
	saved   *model.UserLLMModel
}

func (f *fakeLLMModelRepo) Create(_ context.Context, _ uuid.UUID, in repository.UserLLMModelCreate) (*model.UserLLMModel, error) {
	f.created = &in
	return &model.UserLLMModel{Provider: in.Provider, Kind: in.Kind, Slug: in.Slug, Endpoint: in.Endpoint}, nil
}

func (f *fakeLLMModelRepo) Get(context.Context, uuid.UUID, uuid.UUID) (*model.UserLLMModel, error) {
	if f.saved == nil {
		return nil, repository.ErrLLMModelNotFound
	}
	return f.saved, nil
}

func (f *fakeLLMModelRepo) Update(_ context.Context, _, _ uuid.UUID, slug, _ string) (*model.UserLLMModel, error) {
	f.updated = slug
	return f.saved, nil
}

func TestAIModelCreate(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	notDemo := func(context.Context, uuid.UUID) bool { return false }

	var tested *model.TestUserLLMModelRequest
	pass := func(_ context.Context, req model.TestUserLLMModelRequest) (*model.TestUserLLMModelResponse, error) {
		tested = &req
		return &model.TestUserLLMModelResponse{OK: true}, nil
	}
	fail := func(context.Context, model.TestUserLLMModelRequest) (*model.TestUserLLMModelResponse, error) {
		return &model.TestUserLLMModelResponse{OK: false, Error: "model not found"}, nil
	}

	t.Run("decision model is tested at its endpoint, then saved verified", func(t *testing.T) {
		repo := &fakeLLMModelRepo{}
		svc := NewAIModelService(repo, pass, notDemo)
		_, err := svc.Create(ctx, userID, model.CreateUserLLMModelRequest{
			Provider: model.LLMProviderOpenRouter, Kind: model.LLMModelDecision,
			Slug: " classify-v1 ", Endpoint: "https://decisions.example.com/v1",
		})
		if err != nil {
			t.Fatal(err)
		}
		if tested == nil || tested.Slug != "classify-v1" || tested.Endpoint != "https://decisions.example.com/v1" {
			t.Fatalf("tested = %+v", tested)
		}
		if repo.created == nil || !repo.created.VerifiedAt || repo.created.Slug != "classify-v1" {
			t.Fatalf("created = %+v", repo.created)
		}
	})

	t.Run("a failed test saves nothing", func(t *testing.T) {
		repo := &fakeLLMModelRepo{}
		svc := NewAIModelService(repo, fail, notDemo)
		_, err := svc.Create(ctx, userID, model.CreateUserLLMModelRequest{
			Provider: model.LLMProviderOpenRouter, Kind: model.LLMModelChat, Slug: "nope",
		})
		if !errors.Is(err, ErrAIModelTestFailed) || repo.created != nil {
			t.Fatalf("err = %v, created = %+v", err, repo.created)
		}
	})

	t.Run("endpoint rules", func(t *testing.T) {
		svc := NewAIModelService(&fakeLLMModelRepo{}, pass, notDemo)
		cases := []struct {
			kind     model.LLMModelKind
			endpoint string
			ok       bool
		}{
			{model.LLMModelDecision, "", false},
			{model.LLMModelDecision, "http://decisions.example.com", false}, // the key would go in clear text
			{model.LLMModelDecision, "http://localhost:8080", true},         // self-hosted
			{model.LLMModelDecision, "http://192.168.1.5/d", true},
			{model.LLMModelDecision, "https://user:pw@d.example.com", false},
			{model.LLMModelDecision, "ftp://d.example.com", false},
			{model.LLMModelChat, "https://d.example.com", false}, // chat models take no endpoint
			{model.LLMModelChat, "", true},
		}
		for _, c := range cases {
			_, err := svc.Create(ctx, userID, model.CreateUserLLMModelRequest{
				Provider: model.LLMProviderOpenRouter, Kind: c.kind, Slug: "m", Endpoint: c.endpoint,
			})
			if c.ok != (err == nil) {
				t.Errorf("%s %q: err = %v", c.kind, c.endpoint, err)
			}
		}
	})

	t.Run("bad input and demo are refused", func(t *testing.T) {
		svc := NewAIModelService(&fakeLLMModelRepo{}, pass, notDemo)
		if _, err := svc.Create(ctx, userID, model.CreateUserLLMModelRequest{
			Provider: model.LLMProviderOpenRouter, Kind: "audio", Slug: "m",
		}); !errors.Is(err, ErrInvalidAIModel) {
			t.Fatalf("kind: %v", err)
		}
		if _, err := svc.Create(ctx, userID, model.CreateUserLLMModelRequest{
			Provider: model.LLMProviderAnthropic, Kind: model.LLMModelEmbedding, Slug: "m",
		}); !errors.Is(err, ErrInvalidAIModel) {
			t.Fatalf("embedding on a key that can't serve it: %v", err)
		}
		if _, err := svc.Create(ctx, userID, model.CreateUserLLMModelRequest{
			Provider: model.LLMProviderOllama, Kind: model.LLMModelEmbedding, Slug: "bge-m3", Endpoint: "https://x",
		}); !errors.Is(err, ErrInvalidAIModel) {
			t.Fatalf("embedding with an endpoint: %v", err)
		}
		if _, err := svc.Create(ctx, userID, model.CreateUserLLMModelRequest{
			Provider: "lumo", Kind: model.LLMModelChat, Slug: "m",
		}); !errors.Is(err, ErrInvalidAIProvider) {
			t.Fatalf("provider: %v", err)
		}
		demo := NewAIModelService(&fakeLLMModelRepo{}, pass, func(context.Context, uuid.UUID) bool { return true })
		if _, err := demo.Create(ctx, userID, model.CreateUserLLMModelRequest{
			Provider: model.LLMProviderOpenRouter, Kind: model.LLMModelChat, Slug: "m",
		}); !errors.Is(err, ErrAIProviderReadOnly) {
			t.Fatalf("demo: %v", err)
		}
	})

	t.Run("update re-tests before saving", func(t *testing.T) {
		repo := &fakeLLMModelRepo{saved: &model.UserLLMModel{
			ID: uuid.New(), Provider: model.LLMProviderOpenRouter, Kind: model.LLMModelChat, Slug: "old",
		}}
		if _, err := NewAIModelService(repo, fail, notDemo).Update(ctx, userID, repo.saved.ID,
			model.UpdateUserLLMModelRequest{Slug: "new"}); !errors.Is(err, ErrAIModelTestFailed) || repo.updated != "" {
			t.Fatalf("failed update saved: err %v, updated %q", err, repo.updated)
		}
		if _, err := NewAIModelService(repo, pass, notDemo).Update(ctx, userID, repo.saved.ID,
			model.UpdateUserLLMModelRequest{Slug: "new"}); err != nil || repo.updated != "new" {
			t.Fatalf("update: err %v, updated %q", err, repo.updated)
		}
	})
}
