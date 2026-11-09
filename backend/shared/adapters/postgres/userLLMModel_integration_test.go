package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
)

func TestUserLLMModelRepository(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	keys := NewUserLLMProviderRepository(pool)
	models := NewUserLLMModelRepository(pool)

	userID, budgetID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO auth_users (id) VALUES ($1)`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO budgets (id, user_id, name) VALUES ($1, $2, 'B')`, budgetID, userID); err != nil {
		t.Fatal(err)
	}

	// A model needs the user's key for its provider.
	_, err := models.Create(ctx, userID, repository.UserLLMModelCreate{
		Provider: model.LLMProviderOpenRouter, Kind: model.LLMModelChat, Slug: "x",
	})
	if !errors.Is(err, repository.ErrLLMProviderNotFound) {
		t.Fatalf("create without a key: %v", err)
	}

	for _, p := range []model.LLMProviderName{model.LLMProviderOpenRouter, model.LLMProviderAnthropic} {
		if _, err := keys.Upsert(ctx, userID, repository.UserLLMProviderUpsert{
			Provider: p, APIKeyEncrypted: []byte("sealed"), Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	create := func(p model.LLMProviderName, kind model.LLMModelKind, slug, endpoint string, makeDefault bool) *model.UserLLMModel {
		t.Helper()
		m, err := models.Create(ctx, userID, repository.UserLLMModelCreate{
			Provider: p, Kind: kind, Slug: slug, Endpoint: endpoint, MakeDefault: makeDefault, VerifiedAt: true,
		})
		if err != nil {
			t.Fatalf("create %s: %v", slug, err)
		}
		return m
	}

	// The first model of each kind becomes its default, independently.
	gemini := create(model.LLMProviderOpenRouter, model.LLMModelChat, "google/gemini-2.5-flash", "", false)
	haiku := create(model.LLMProviderAnthropic, model.LLMModelChat, "claude-haiku", "", false)
	classify := create(model.LLMProviderOpenRouter, model.LLMModelDecision, "classify-v1", "https://d.example.com", false)
	if !gemini.IsDefault || haiku.IsDefault || !classify.IsDefault {
		t.Fatalf("defaults: gemini=%v haiku=%v classify=%v", gemini.IsDefault, haiku.IsDefault, classify.IsDefault)
	}
	if gemini.LastVerifiedAt == nil || classify.Endpoint != "https://d.example.com" || classify.Provider != model.LLMProviderOpenRouter {
		t.Fatalf("created = %+v / %+v", gemini, classify)
	}

	// The same key, kind and slug can't be saved twice.
	_, err = models.Create(ctx, userID, repository.UserLLMModelCreate{
		Provider: model.LLMProviderOpenRouter, Kind: model.LLMModelChat, Slug: "google/gemini-2.5-flash",
	})
	if !errors.Is(err, repository.ErrLLMModelExists) {
		t.Fatalf("duplicate: %v", err)
	}

	// SetDefault moves the default within the kind only.
	if _, err := models.SetDefault(ctx, userID, haiku.ID); err != nil {
		t.Fatal(err)
	}
	list, err := models.ListByUser(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	defaults := map[uuid.UUID]bool{}
	for _, m := range list {
		defaults[m.ID] = m.IsDefault
	}
	if !defaults[haiku.ID] || defaults[gemini.ID] || !defaults[classify.ID] {
		t.Fatalf("after SetDefault: %v", defaults)
	}

	// Usable models carry the key and come default first.
	usable, err := models.ListUsableForBudget(ctx, budgetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(usable) != 3 || !usable[0].IsDefault || string(usable[0].APIKeyEncrypted) != "sealed" {
		t.Fatalf("usable = %+v", usable)
	}

	// Filtering by kind leaves out the others.
	decisionsOnly, err := models.ListUsableForBudget(ctx, budgetID, model.LLMModelDecision)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisionsOnly) != 1 || decisionsOnly[0].ID != classify.ID {
		t.Fatalf("decision models = %+v", decisionsOnly)
	}

	// New models are ordered last, and Reorder changes the order.
	if haiku.Priority != 1 && gemini.Priority != 1 {
		t.Fatalf("priorities: haiku %d, gemini %d", haiku.Priority, gemini.Priority)
	}
	if err := models.Reorder(ctx, userID, model.LLMModelChat, []uuid.UUID{gemini.ID, haiku.ID}); err != nil {
		t.Fatal(err)
	}
	if g, _ := models.Get(ctx, userID, gemini.ID); g == nil || g.Priority != 0 {
		t.Fatalf("gemini after reorder: %+v", g)
	}
	// A partial list changes nothing.
	if err := models.Reorder(ctx, userID, model.LLMModelChat, []uuid.UUID{haiku.ID}); !errors.Is(err, repository.ErrLLMModelNotFound) {
		t.Fatalf("partial reorder: %v", err)
	}
	if g, _ := models.Get(ctx, userID, gemini.ID); g == nil || g.Priority != 0 {
		t.Fatalf("partial reorder was applied: %+v", g)
	}

	// A disabled model isn't usable, and comes back when enabled.
	if _, err := models.SetEnabled(ctx, userID, classify.ID, false); err != nil {
		t.Fatal(err)
	}
	decisionsOnly, err = models.ListUsableForBudget(ctx, budgetID, model.LLMModelDecision)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisionsOnly) != 0 {
		t.Fatalf("disabled model listed: %+v", decisionsOnly)
	}
	if m, err := models.SetEnabled(ctx, userID, classify.ID, true); err != nil || !m.Enabled {
		t.Fatalf("enable: %+v, %v", m, err)
	}

	// Deleting the default promotes the first remaining model of its kind.
	if err := models.Delete(ctx, userID, haiku.ID); err != nil {
		t.Fatal(err)
	}
	if g, _ := models.Get(ctx, userID, gemini.ID); g == nil || !g.IsDefault {
		t.Fatalf("gemini not promoted: %+v", g)
	}

	// Deleting a key deletes its models.
	if err := keys.Delete(ctx, userID, model.LLMProviderOpenRouter); err != nil {
		t.Fatal(err)
	}
	if list, _ := models.ListByUser(ctx, userID); len(list) != 0 {
		t.Fatalf("models left after deleting the key: %+v", list)
	}

	// Another user can't touch these rows.
	if _, err := models.Get(ctx, uuid.New(), classify.ID); !errors.Is(err, repository.ErrLLMModelNotFound) {
		t.Fatalf("foreign get: %v", err)
	}
}
