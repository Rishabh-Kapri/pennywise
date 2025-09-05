package repository

import (
	"context"
	"testing"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func getTestDbConn(ctx context.Context) (*pgxpool.Pool, error) {
	testDbUrl := "postgres://admin:admin@192.168.1.34:5433/testdb?sslmode=disable"
	dbpool, err := pgxpool.New(ctx, testDbUrl)
	return dbpool, err
}

func addDummyData(t *testing.T, dbpool *pgxpool.Pool, ctx context.Context, budgetId uuid.UUID, catId uuid.UUID) {
	_, err := dbpool.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS monthly_budgets (
			budget_id UUID,
			category_id UUID,
			month TEXT,
			budgeted NUMERIC(12, 2),
			carryover_balance NUMERIC(12, 2),
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)`)
	require.NoError(t, err)

	dummyRows := [][]any{
		{"2025-05", budgetId, catId, 1000.00, 0.00},     // activity: -1000
		{"2025-06", budgetId, catId, 2500.00, 0.00},     // activity: -2500
		{"2025-07", budgetId, catId, 2000.00, -5000.00}, // activity: -7000
		{"2025-08", budgetId, catId, 1000.00, 500.00},   // activity: 4500
	}
	for _, row := range dummyRows {
		_, err = dbpool.Exec(ctx, `
			INSERT INTO monthly_budgets (budget_id, category_id, month, budgeted, carryover_balance, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		`, row[1], row[2], row[0], row[3], row[4])
	}
}

func TestCreate(t *testing.T) {
	t.Run("New Month When Previous Month Exists", func(t *testing.T) {
		ctx := context.Background()
		dbpool, err := getTestDbConn(ctx)
		require.NoError(t, err)

		defer dbpool.Close()

		budgetId := uuid.New()
		catId := uuid.New()

		addDummyData(t, dbpool, ctx, budgetId, catId)

		// Cleanup even after assestions fail
		defer func() {
			_, _ = dbpool.Exec(ctx, `DELETE FROM monthly_budgets WHERE budget_id = $1`, budgetId)
		}()

		monthlyBudget := model.MonthlyBudget{
			Month:      "2025-09",
			BudgetID:   budgetId,
			CategoryID: catId,
			Budgeted:   500.00,
		}

		repo := repository.NewMonthlyBudgetRepository(dbpool)

		err = repo.Create(ctx, budgetId, monthlyBudget)
		require.NoError(t, err)

		var gotBudgeted, gotCarryover float64

		err = dbpool.QueryRow(ctx, `
			SELECT budgeted, carryover_balance FROM monthly_budgets WHERE budget_id = $1 AND category_id = $2 AND month = $3
			`, budgetId, catId, "2025-09",
		).Scan(&gotBudgeted, &gotCarryover)
		require.NoError(t, err)
		require.Equal(t, 500.00, gotBudgeted)
		require.Equal(t, 1000.00, gotCarryover)
	})

	t.Run("New Month When Previous Month Does Not Exist", func(t *testing.T) {
		ctx := context.Background()
		dbpool, err := getTestDbConn(ctx)
		require.NoError(t, err)

		defer dbpool.Close()

		budgetId := uuid.New()
		catId := uuid.New()

		addDummyData(t, dbpool, ctx, budgetId, catId)

		// Cleanup even after assestions fail
		defer func() {
			_, _ = dbpool.Exec(ctx, `DELETE FROM monthly_budgets WHERE budget_id = $1`, budgetId)
		}()

		monthlyBudget := model.MonthlyBudget{
			Month:      "2025-04",
			BudgetID:   budgetId,
			CategoryID: catId,
			Budgeted:   500.00,
		}

		repo := repository.NewMonthlyBudgetRepository(dbpool)

		err = repo.Create(ctx, budgetId, monthlyBudget)
		require.NoError(t, err)

		var gotBudgeted, gotCarryover float64

		err = dbpool.QueryRow(ctx, `
			SELECT budgeted, carryover_balance FROM monthly_budgets WHERE budget_id = $1 AND category_id = $2 AND month = $3
			`, budgetId, catId, "2025-04",
		).Scan(&gotBudgeted, &gotCarryover)
		require.NoError(t, err)
		require.Equal(t, 500.00, gotBudgeted)
		require.Equal(t, 500.00, gotCarryover)
	})
}

func TestMonthlyBudgetRepo_UpdateByCatIdAndMonth(t *testing.T) {
	ctx := context.Background()
	dbpool, err := getTestDbConn(ctx)
	require.NoError(t, err)

	defer dbpool.Close()

	budgetId := uuid.New()
	catId := uuid.New()

	addDummyData(t, dbpool, ctx, budgetId, catId)

	// Cleanup even after assestions fail
	defer func() {
		_, _ = dbpool.Exec(ctx, `DELETE FROM monthly_budgets WHERE budget_id = $1`, budgetId)
	}()

	repo := repository.NewMonthlyBudgetRepository(dbpool)

	err = repo.UpdateBudgetedByCatIdAndMonth(context.Background(), budgetId, catId, "2025-08", 500)
	require.NoError(t, err)

	var gotBudgeted, gotCarryover float64

	err = dbpool.QueryRow(ctx, `
		SELECT budgeted, carryover_balance FROM monthly_budgets WHERE budget_id = $1 AND category_id = $2 AND month = $3
	`, budgetId, catId, "2025-08",
	).Scan(&gotBudgeted, &gotCarryover)
	require.NoError(t, err)
	require.Equal(t, 500.00, gotBudgeted)
	require.Equal(t, 0.00, gotCarryover)
}
