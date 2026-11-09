package postgres

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
)

func TestPipelineStepRepositoryStoresFallback(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewPipelineStepRepository(pool)

	userID, budgetID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO auth_users (id) VALUES ($1)`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO budgets (id, user_id, name) VALUES ($1, $2, 'B')`, budgetID, userID); err != nil {
		t.Fatal(err)
	}

	on := true
	for _, p := range []string{"message", "statement"} {
		if _, err := repo.Upsert(ctx, model.PipelineStepOverride{
			BudgetID: budgetID, Pipeline: p, Step: "check", Fallback: &on,
		}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.ListByBudgetAndPipeline(ctx, budgetID, "message")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Pipeline != "message" || got[0].Fallback == nil || !*got[0].Fallback {
		t.Fatalf("overrides = %+v", got)
	}

	// Saving without it goes back to following the default.
	saved, err := repo.Upsert(ctx, model.PipelineStepOverride{BudgetID: budgetID, Pipeline: "message", Step: "check"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Fallback != nil {
		t.Fatalf("fallback after clearing = %v", *saved.Fallback)
	}
}
