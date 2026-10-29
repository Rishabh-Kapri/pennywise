package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// fakeLLMProviderRepo serves ListByUser and records the last upsert.
type fakeLLMProviderRepo struct {
	repository.UserLLMProviderRepository
	existing []model.UserLLMProvider
	upserted *repository.UserLLMProviderUpsert
}

func (f *fakeLLMProviderRepo) ListByUser(context.Context, uuid.UUID) ([]model.UserLLMProvider, error) {
	return f.existing, nil
}

func (f *fakeLLMProviderRepo) Upsert(
	_ context.Context,
	_ uuid.UUID,
	in repository.UserLLMProviderUpsert,
) (*model.UserLLMProvider, error) {
	f.upserted = &in
	return &model.UserLLMProvider{Provider: in.Provider, Model: in.Model}, nil
}

func newTestAIProviderService(t *testing.T, repo *fakeLLMProviderRepo, demo bool) (AIProviderService, *utils.SecretBox) {
	t.Helper()
	box, err := utils.NewSecretBox(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	if err != nil {
		t.Fatal(err)
	}
	isDemo := func(context.Context, uuid.UUID) bool { return demo }
	return NewAIProviderService(repo, box, nil, isDemo), box
}

func TestAIProviderUpsert(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	key := func(s string) *string { return &s }

	t.Run("seals a new key and keeps only a hint", func(t *testing.T) {
		repo := &fakeLLMProviderRepo{}
		svc, box := newTestAIProviderService(t, repo, false)
		_, err := svc.Upsert(ctx, userID, model.LLMProviderOpenAI, model.UpsertUserLLMProviderRequest{
			APIKey: key("sk-test-abcd1234"), Model: "gpt-5-mini",
		})
		if err != nil {
			t.Fatal(err)
		}
		if repo.upserted.APIKeyHint != "1234" || repo.upserted.KeepAPIKey {
			t.Fatalf("upsert = %+v", repo.upserted)
		}
		plain, err := box.Open(repo.upserted.APIKeyEncrypted)
		if err != nil || plain != "sk-test-abcd1234" {
			t.Fatalf("Open = %q, %v", plain, err)
		}
	})

	t.Run("hosted provider needs a key", func(t *testing.T) {
		svc, _ := newTestAIProviderService(t, &fakeLLMProviderRepo{}, false)
		_, err := svc.Upsert(ctx, userID, model.LLMProviderAnthropic, model.UpsertUserLLMProviderRequest{Model: "claude-haiku-5-5"})
		if !errors.Is(err, ErrAIProviderKeyRequired) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("omitted key keeps the saved one", func(t *testing.T) {
		repo := &fakeLLMProviderRepo{existing: []model.UserLLMProvider{
			{Provider: model.LLMProviderOpenRouter, HasAPIKey: true, Priority: 2},
		}}
		svc, _ := newTestAIProviderService(t, repo, false)
		_, err := svc.Upsert(ctx, userID, model.LLMProviderOpenRouter, model.UpsertUserLLMProviderRequest{Model: "x/y"})
		if err != nil {
			t.Fatal(err)
		}
		if !repo.upserted.KeepAPIKey || repo.upserted.Priority != 2 {
			t.Fatalf("upsert = %+v", repo.upserted)
		}
	})

	t.Run("ollama needs a URL but no key", func(t *testing.T) {
		repo := &fakeLLMProviderRepo{}
		svc, _ := newTestAIProviderService(t, repo, false)
		_, err := svc.Upsert(ctx, userID, model.LLMProviderOllama, model.UpsertUserLLMProviderRequest{Model: "gemma"})
		if !errors.Is(err, ErrInvalidAIProvider) {
			t.Fatalf("missing URL err = %v", err)
		}
		_, err = svc.Upsert(ctx, userID, model.LLMProviderOllama, model.UpsertUserLLMProviderRequest{
			Model: "gemma", BaseURL: "https://ollama.example.com/",
		})
		if err != nil || repo.upserted.BaseURL != "https://ollama.example.com" {
			t.Fatalf("err = %v, upsert = %+v", err, repo.upserted)
		}
	})

	t.Run("base URL only for ollama", func(t *testing.T) {
		svc, _ := newTestAIProviderService(t, &fakeLLMProviderRepo{}, false)
		_, err := svc.Upsert(ctx, userID, model.LLMProviderOpenAI, model.UpsertUserLLMProviderRequest{
			APIKey: key("sk-test-abcd1234"), Model: "m", BaseURL: "https://evil.example.com",
		})
		if !errors.Is(err, ErrInvalidAIProvider) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("unknown provider and demo user are refused", func(t *testing.T) {
		svc, _ := newTestAIProviderService(t, &fakeLLMProviderRepo{}, false)
		if _, err := svc.Upsert(ctx, userID, "lumo", model.UpsertUserLLMProviderRequest{Model: "m"}); !errors.Is(err, ErrInvalidAIProvider) {
			t.Fatalf("err = %v", err)
		}
		demoSvc, _ := newTestAIProviderService(t, &fakeLLMProviderRepo{}, true)
		if _, err := demoSvc.Upsert(ctx, userID, model.LLMProviderOpenAI, model.UpsertUserLLMProviderRequest{}); !errors.Is(err, ErrAIProviderReadOnly) {
			t.Fatalf("err = %v", err)
		}
	})
}
