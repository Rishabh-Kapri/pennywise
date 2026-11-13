package userllm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/resolver"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

type fakeRepo struct {
	repository.UserLLMModelRepository
	rows     []model.UserLLMModel
	recorded map[uuid.UUID]string
}

func (f *fakeRepo) ListUsableForBudget(
	_ context.Context,
	_ uuid.UUID,
	kinds ...model.LLMModelKind,
) ([]model.UserLLMModel, error) {
	if len(kinds) == 0 {
		return f.rows, nil
	}
	var out []model.UserLLMModel
	for _, row := range f.rows {
		if slices.Contains(kinds, row.Kind) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeRepo) RecordVerification(_ context.Context, _, id uuid.UUID, testErr string) error {
	if f.recorded == nil {
		f.recorded = map[uuid.UUID]string{}
	}
	f.recorded[id] = testErr
	return nil
}

type fakeKeys struct {
	repository.UserLLMProviderRepository
	keys map[model.LLMProviderName]model.UserLLMProvider
}

func (f *fakeKeys) GetForBudget(_ context.Context, _ uuid.UUID, p model.LLMProviderName) (*model.UserLLMProvider, error) {
	k, ok := f.keys[p]
	if !ok {
		return nil, repository.ErrLLMProviderNotFound
	}
	return &k, nil
}

func newTestResolver(t *testing.T, repo *fakeRepo) (*Resolver, *utils.SecretBox) {
	return newTestResolverWith(t, repo, &fakeKeys{}, false)
}

func newTestResolverWith(t *testing.T, repo *fakeRepo, keys *fakeKeys, allowPrivate bool) (*Resolver, *utils.SecretBox) {
	t.Helper()
	box, err := utils.NewSecretBox(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	if err != nil {
		t.Fatal(err)
	}
	tel, err := otelSDK.NewTelemetry(context.Background(), otelSDK.Config{ServiceName: "test", OtelSdkDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	clients := resolver.NewResolver(box, transport.NewUserURLClient(allowPrivate), tel)
	return New(repo, keys, clients, tel), box
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
		repo.rows = []model.UserLLMModel{
			{ID: uuid.New(), Kind: model.LLMModelChat, Provider: model.LLMProviderOpenRouter,
				Slug: "google/gemini-2.5-flash", IsDefault: true, APIKeyEncrypted: sealed(t, box, "or-key")},
			// Decision models never become chat clients.
			{ID: uuid.New(), Kind: model.LLMModelDecision, Provider: model.LLMProviderOpenAI,
				Slug: "classify", Endpoint: "https://d.example.com", APIKeyEncrypted: sealed(t, box, "oa-key")},
			{ID: uuid.New(), Kind: model.LLMModelChat, Provider: model.LLMProviderAnthropic,
				Slug: "claude-haiku-4-5", APIKeyEncrypted: sealed(t, box, "ant-key")},
			// A second chat model on the same key: the key's first model is used.
			{ID: uuid.New(), Kind: model.LLMModelChat, Provider: model.LLMProviderAnthropic,
				Slug: "claude-sonnet", APIKeyEncrypted: sealed(t, box, "ant-key")},
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
		repo.rows = []model.UserLLMModel{
			{ID: uuid.New(), Kind: model.LLMModelChat, Provider: model.LLMProviderOpenAI, Slug: "gpt-4o-mini",
				APIKeyEncrypted: []byte("garbage")},
			{ID: id, Kind: model.LLMModelChat, Provider: model.LLMProviderAnthropic, Slug: "old",
				APIKeyEncrypted: sealed(t, box, "k"), UpdatedAt: time.Unix(1, 0)},
		}
		if _, m, err := r.Resolve(ctx, "", ""); err != nil || m != "old" {
			t.Fatalf("model %q, err %v", m, err)
		}

		repo.rows[1].Slug = "new"
		repo.rows[1].UpdatedAt = time.Unix(2, 0)
		if _, m, _ := r.Resolve(ctx, "", ""); m != "new" {
			t.Fatalf("edited row not picked up, model %q", m)
		}
	})
}

func TestDecisionModelTest(t *testing.T) {
	ctx := utils.WithBudgetID(context.Background(), uuid.New())

	var gotAuth, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"classify-v1","answers":{"ok":{"type":"noul","noul":0.97}},"usage":{}}`))
	}))
	defer srv.Close()

	repo := &fakeRepo{}
	keys := &fakeKeys{keys: map[model.LLMProviderName]model.UserLLMProvider{}}
	// The fake endpoint is on 127.0.0.1, so allow private addresses here.
	r, box := newTestResolverWith(t, repo, keys, true)
	keys.keys[model.LLMProviderOpenRouter] = model.UserLLMProvider{
		Provider: model.LLMProviderOpenRouter, APIKeyEncrypted: sealed(t, box, "or-key"),
	}

	modelID := uuid.New()
	res, err := r.Test(ctx, model.TestUserLLMModelRequest{
		ModelID: &modelID, Provider: model.LLMProviderOpenRouter, Kind: model.LLMModelDecision,
		Slug: "classify-v1", Endpoint: srv.URL,
	})
	if err != nil || !res.OK || res.Model != "classify-v1" {
		t.Fatalf("decision test = %+v, %v", res, err)
	}
	if gotAuth != "Bearer or-key" || gotModel != "classify-v1" {
		t.Fatalf("endpoint got auth %q, model %q", gotAuth, gotModel)
	}
	if e, ok := repo.recorded[modelID]; !ok || e != "" {
		t.Fatalf("verification not recorded as passed: %v %q", ok, e)
	}

	// Without allowPrivate the same endpoint is refused, and that's a failed
	// test, not an error.
	strict, _ := newTestResolverWith(t, &fakeRepo{}, keys, false)
	keys.keys[model.LLMProviderOpenRouter] = model.UserLLMProvider{
		Provider: model.LLMProviderOpenRouter, APIKeyEncrypted: sealed(t, box, "or-key"),
	}
	res, err = strict.Test(ctx, model.TestUserLLMModelRequest{
		Provider: model.LLMProviderOpenRouter, Kind: model.LLMModelDecision, Slug: "x", Endpoint: srv.URL,
	})
	if err != nil || res.OK {
		t.Fatalf("private endpoint should fail the test: %+v, %v", res, err)
	}

	// A provider without a saved key is an error.
	if _, err := r.Test(ctx, model.TestUserLLMModelRequest{
		Provider: model.LLMProviderAnthropic, Kind: model.LLMModelChat, Slug: "x",
	}); !errors.Is(err, repository.ErrLLMProviderNotFound) {
		t.Fatalf("missing key: %v", err)
	}

}
