package postgres

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Seeded: "Eating Out" and "Groceries" (see newSeed), plus a system inflow
// category and a credit-card group the budget's metadata points at. Eating Out
// has two budgeted months and one transaction.
func TestCategoryReads(t *testing.T) {
	pool, txns, s := setup(t)
	repo := NewCategoryRepository(pool)
	ctx := context.Background()

	inflowID, ccGroupID, ccCatID := uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO categories (id, name, budget_id, is_system) VALUES ($1, 'Inflow', $2, TRUE)`, []any{inflowID, s.budgetID}},
		{`INSERT INTO category_groups (id, name, budget_id) VALUES ($1, 'Credit Cards', $2)`, []any{ccGroupID, s.budgetID}},
		{`INSERT INTO categories (id, name, budget_id, category_group_id) VALUES ($1, 'HDFC Card', $2, $3)`, []any{ccCatID, s.budgetID, ccGroupID}},
		{`UPDATE budgets SET metadata = jsonb_build_object('inflowCategoryId', $1::text, 'ccGroupId', $2::text) WHERE id = $3`, []any{inflowID, ccGroupID, s.budgetID}},
		{`INSERT INTO monthly_budgets (budget_id, category_id, month, budgeted, carryover_balance) VALUES ($1, $2, '2026-10', 1000, 1000)`, []any{s.budgetID, s.categoryID}},
		{`INSERT INTO monthly_budgets (budget_id, category_id, month, budgeted, carryover_balance) VALUES ($1, $2, '2026-11', 500, 1100)`, []any{s.budgetID, s.categoryID}},
	} {
		_, err := pool.Exec(ctx, q.sql, q.args...)
		require.NoError(t, err)
	}
	mustCreate(t, txns, newTxn(s, "2026-10-12", -400))

	t.Run("GetAll carries each category's budgeted, activity and balance by month", func(t *testing.T) {
		all, err := repo.GetAll(ctx, s.budgetID)
		require.NoError(t, err)
		require.Len(t, all, 4)
		for _, c := range all {
			if c.ID != s.categoryID {
				continue
			}
			require.Equal(t, "Eating Out", c.Name)
			require.Equal(t, map[string]float64{"2026-10": 1000, "2026-11": 500}, c.Budgeted)
			require.Equal(t, map[string]float64{"2026-10": -400}, c.Activity)
			require.Equal(t, map[string]float64{"2026-10": 1000, "2026-11": 1100}, c.Balance)
			return
		}
		t.Fatal("seeded category missing")
	})

	t.Run("GetById matches GetAll", func(t *testing.T) {
		one, err := repo.GetById(ctx, s.budgetID, s.categoryID)
		require.NoError(t, err)
		require.Equal(t, map[string]float64{"2026-10": -400}, one.Activity)
		require.Equal(t, map[string]float64{"2026-10": 1000, "2026-11": 500}, one.Budgeted)
	})

	t.Run("Search matches by name", func(t *testing.T) {
		found, err := repo.Search(ctx, s.budgetID, "Grocer")
		require.NoError(t, err)
		require.Len(t, found, 1)
		require.Equal(t, s.category2, found[0].ID)
	})

	t.Run("simplified lists skip the inflow and credit-card categories", func(t *testing.T) {
		simple, err := repo.GetAllSimplified(ctx, s.budgetID)
		require.NoError(t, err)
		names := []string{}
		for _, c := range simple {
			names = append(names, c.Name)
		}
		require.ElementsMatch(t, []string{"Eating Out", "Groceries"}, names)

		closest, err := repo.FindClosestSimplified(ctx, s.budgetID, "Grocery", 5)
		require.NoError(t, err)
		require.NotEmpty(t, closest)
		require.Equal(t, s.category2, closest[0].ID)
		for _, m := range closest {
			require.NotEqual(t, inflowID, m.ID)
			require.NotEqual(t, ccCatID, m.ID)
		}
	})
}

func TestCategoryAndGroupDelete(t *testing.T) {
	pool, _, s := setup(t)
	categories := NewCategoryRepository(pool)
	groups := NewCategoryGroupRepository(pool)
	ctx := context.Background()

	require.NoError(t, categories.DeleteById(ctx, s.budgetID, s.categoryID))
	_, err := categories.GetById(ctx, s.budgetID, s.categoryID)
	require.ErrorIs(t, err, repository.ErrNotFound, "a deleted category is no longer readable")
	require.NoError(t, categories.DeleteById(ctx, s.budgetID, s.category2), "other categories are unaffected")

	missing := uuid.New()
	require.ErrorIs(t, categories.DeleteById(ctx, s.budgetID, missing), repository.ErrNotFound)
	require.ErrorIs(t, groups.DeleteById(ctx, s.budgetID, missing), repository.ErrNotFound)

	otherBudget := newSeed(t, pool)
	require.ErrorIs(t, categories.DeleteById(ctx, otherBudget.budgetID, s.categoryID), repository.ErrNotFound,
		"a category can't be deleted through another budget")
}
