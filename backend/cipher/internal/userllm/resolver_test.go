package userllm

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

type fakeRepo struct {
	repository.UserLLMProviderRepository
	rows []model.UserLLMProvider
}

func (f *fakeRepo) ListEnabledForBudget(context.Context, uuid.UUID) ([]model.UserLLMProvider, error) {
	return f.rows, nil
}

func newTestResolver(t *testing.T, repo *fakeRepo) (*Resolver, *utils.SecretBox) {
	t.Helper()
	box, err := utils.NewSecretBox(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	if err != nil {
		t.Fatal(err)
	}
	tel, err := otelSDK.NewTelemetry(context.Background(), otelSDK.Config{ServiceName: "test", OtelSdkDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return New(repo, box, tel, false), box
}

func sealed(t *testing.T, box *utils.SecretBox, key string) []byte {
	t.Helper()
	b, err := box.Seal(key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestResolver(t *testing.T) {
	ctx := utils.WithBudgetID(context.Background(), uuid.New())

	t.Run("no providers is not configured", func(t *testing.T) {
		r, _ := newTestResolver(t, &fakeRepo{})
		if _, _, err := r.Resolve(ctx, "", ""); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("Resolve err = %v", err)
		}
		if _, err := r.Targets(ctx); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("Targets err = %v", err)
		}
	})

	t.Run("no budget in context is an error", func(t *testing.T) {
		r, _ := newTestResolver(t, &fakeRepo{})
		if _, _, err := r.Resolve(context.Background(), "", ""); err == nil {
			t.Fatal("expected an error without a budget")
		}
	})

	t.Run("chain order, requested model and fallback to primary", func(t *testing.T) {
		repo := &fakeRepo{}
		r, box := newTestResolver(t, repo)
		repo.rows = []model.UserLLMProvider{
			{ID: uuid.New(), Provider: model.LLMProviderOpenRouter, Model: "google/gemini-2.5-flash",
				APIKeyEncrypted: sealed(t, box, "or-key")},
			{ID: uuid.New(), Provider: model.LLMProviderAnthropic, Model: "claude-haiku-4-5",
				APIKeyEncrypted: sealed(t, box, "ant-key")},
		}

		targets, err := r.Targets(ctx)
		if err != nil || len(targets) != 2 || targets[0].Provider != "openrouter" || targets[1].Model != "claude-haiku-4-5" {
			t.Fatalf("Targets = %+v, %v", targets, err)
		}

		if _, m, err := r.Resolve(ctx, "anthropic", "claude-sonnet-4-6"); err != nil || m != "claude-sonnet-4-6" {
			t.Fatalf("configured provider: model %q, err %v", m, err)
		}
		// lumo isn't one of the user's: use their primary with its own model,
		// not the model asked for.
		if _, m, err := r.Resolve(ctx, "lumo", "lumo-max"); err != nil || m != "google/gemini-2.5-flash" {
			t.Fatalf("fallback: model %q, err %v", m, err)
		}
	})

	t.Run("undecryptable row is skipped, edits rebuild", func(t *testing.T) {
		repo := &fakeRepo{}
		r, box := newTestResolver(t, repo)
		id := uuid.New()
		repo.rows = []model.UserLLMProvider{
			{ID: uuid.New(), Provider: model.LLMProviderOpenAI, Model: "gpt-4o-mini", APIKeyEncrypted: []byte("garbage")},
			{ID: id, Provider: model.LLMProviderAnthropic, Model: "old", APIKeyEncrypted: sealed(t, box, "k"), UpdatedAt: time.Unix(1, 0)},
		}
		if _, m, err := r.Resolve(ctx, "", ""); err != nil || m != "old" {
			t.Fatalf("model %q, err %v", m, err)
		}

		repo.rows[1].Model = "new"
		repo.rows[1].UpdatedAt = time.Unix(2, 0)
		if _, m, _ := r.Resolve(ctx, "", ""); m != "new" {
			t.Fatalf("edited row not picked up, model %q", m)
		}
	})
}
