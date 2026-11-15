package postgres

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type monthAmounts struct {
	Budgeted         float64
	CarryoverBalance float64
}

type monthlyBudgetFixture struct {
	repo     repository.MonthlyBudgetRepository
	pool     *pgxpool.Pool
	budgetID uuid.UUID
	catID    uuid.UUID
}

// newMonthlyBudgetFixture seeds one category with four months:
// 2025-05 budgeted 1000 carryover 0, 2025-06 2500/0, 2025-07 2000/-5000,
// 2025-08 1000/500.
func newMonthlyBudgetFixture(t *testing.T) monthlyBudgetFixture {
	t.Helper()
	pool := testPool(t)
	s := newSeed(t, pool)
	rows := []struct {
		month           string
		budgeted, carry float64
	}{
		{"2025-05", 1000, 0}, {"2025-06", 2500, 0}, {"2025-07", 2000, -5000}, {"2025-08", 1000, 500},
	}
	for _, r := range rows {
		_, err := pool.Exec(context.Background(), `
			INSERT INTO monthly_budgets (budget_id, category_id, month, budgeted, carryover_balance)
			VALUES ($1, $2, $3, $4, $5)`, s.budgetID, s.categoryID, r.month, r.budgeted, r.carry)
		require.NoError(t, err)
	}
	return monthlyBudgetFixture{repo: NewMonthlyBudgetRepository(pool), pool: pool, budgetID: s.budgetID, catID: s.categoryID}
}

func (f monthlyBudgetFixture) months(t *testing.T) map[string]monthAmounts {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `
		SELECT month, budgeted, carryover_balance FROM monthly_budgets
		WHERE budget_id = $1 AND category_id = $2`, f.budgetID, f.catID)
	require.NoError(t, err)
	defer rows.Close()
	got := map[string]monthAmounts{}
	for rows.Next() {
		var month string
		var a monthAmounts
		require.NoError(t, rows.Scan(&month, &a.Budgeted, &a.CarryoverBalance))
		got[month] = a
	}
	require.NoError(t, rows.Err())
	return got
}

func TestMonthlyBudgetGetByCatIdAndMonth(t *testing.T) {
	f := newMonthlyBudgetFixture(t)
	ctx := context.Background()

	mb, err := f.repo.GetByCatIdAndMonth(ctx, f.budgetID, f.catID, "2025-06")
	require.NoError(t, err)
	require.Equal(t, 2500.0, mb.Budgeted)
	require.Equal(t, 0.0, mb.CarryoverBalance)

	mb, err = f.repo.GetByCatIdAndMonth(ctx, f.budgetID, f.catID, "2020-01")
	require.ErrorIs(t, err, repository.ErrNotFound)
	require.Nil(t, mb)
}

func TestMonthlyBudgetCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("a month after the data carries the last carryover forward", func(t *testing.T) {
		f := newMonthlyBudgetFixture(t)
		require.NoError(t, f.repo.Create(ctx, f.budgetID, model.MonthlyBudget{CategoryID: f.catID, Month: "2025-09", Budgeted: 500}))
		require.Equal(t, monthAmounts{500, 1000}, f.months(t)["2025-09"])
	})

	t.Run("a month before the data pushes its budget into every later month", func(t *testing.T) {
		f := newMonthlyBudgetFixture(t)
		require.NoError(t, f.repo.Create(ctx, f.budgetID, model.MonthlyBudget{CategoryID: f.catID, Month: "2025-04", Budgeted: 500}))
		require.Equal(t, map[string]monthAmounts{
			"2025-04": {500, 500},
			"2025-05": {1000, 500},
			"2025-06": {2500, 500},
			"2025-07": {2000, -4500},
			"2025-08": {1000, 1000},
		}, f.months(t))
	})

	t.Run("an existing month is refused and left alone", func(t *testing.T) {
		f := newMonthlyBudgetFixture(t)
		before := f.months(t)
		require.Error(t, f.repo.Create(ctx, f.budgetID, model.MonthlyBudget{CategoryID: f.catID, Month: "2025-06", Budgeted: 500}))
		require.Equal(t, before, f.months(t))
	})
}

func TestMonthlyBudgetUpdateBudgeted(t *testing.T) {
	ctx := context.Background()

	t.Run("the change flows into this month and every later one", func(t *testing.T) {
		f := newMonthlyBudgetFixture(t)
		require.NoError(t, f.repo.UpdateBudgetedByCatIdAndMonth(ctx, f.budgetID, f.catID, "2025-07", 5000))
		require.Equal(t, map[string]monthAmounts{
			"2025-05": {1000, 0},
			"2025-06": {2500, 0},
			"2025-07": {5000, -2000},
			"2025-08": {1000, 3500},
		}, f.months(t))
	})

	t.Run("a missing month is ErrNotFound", func(t *testing.T) {
		f := newMonthlyBudgetFixture(t)
		err := f.repo.UpdateBudgetedByCatIdAndMonth(ctx, f.budgetID, f.catID, "2025-09", 5000)
		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestMonthlyBudgetUpdateCarryover(t *testing.T) {
	ctx := context.Background()

	t.Run("the amount is added to this month and every later one", func(t *testing.T) {
		f := newMonthlyBudgetFixture(t)
		err := NewDBTxn(f.pool).Do(ctx, func(ctx context.Context) error {
			return f.repo.UpdateCarryoverByCatIdAndMonth(ctx, f.budgetID, f.catID, "2025-07", 1000)
		})
		require.NoError(t, err)
		require.Equal(t, map[string]monthAmounts{
			"2025-05": {1000, 0},
			"2025-06": {2500, 0},
			"2025-07": {2000, -4000},
			"2025-08": {1000, 1500},
		}, f.months(t))
	})

	t.Run("a missing month is ErrNotFound", func(t *testing.T) {
		f := newMonthlyBudgetFixture(t)
		err := f.repo.UpdateCarryoverByCatIdAndMonth(ctx, f.budgetID, f.catID, "2025-09", 1000)
		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}
