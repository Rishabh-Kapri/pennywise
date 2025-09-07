package repository

import (
	"context"
	"testing"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Test data structures
type budgetData struct {
	Budgeted         float64
	CarryoverBalance float64
}

type testSuite struct {
	ctx      context.Context
	dbpool   *pgxpool.Pool
	repo     MonthlyBudgetRepository
	budgetID uuid.UUID
	catID    uuid.UUID
}

// Database connection and setup utilities
func getTestDbConn(ctx context.Context) (*pgxpool.Pool, error) {
	testDbUrl := "postgres://admin:admin@192.168.1.34:5433/testdb?sslmode=disable"
	dbpool, err := pgxpool.New(ctx, testDbUrl)
	return dbpool, err
}

func setupTestSuite(t *testing.T) *testSuite {
	ctx := context.Background()
	dbpool, err := getTestDbConn(ctx)
	require.NoError(t, err)

	budgetID := uuid.New()
	catID := uuid.New()

	return &testSuite{
		ctx:      ctx,
		dbpool:   dbpool,
		repo:     NewMonthlyBudgetRepository(dbpool),
		budgetID: budgetID,
		catID:    catID,
	}
}

func (ts *testSuite) tearDown() {
	ts.dbpool.Close()
}

func (ts *testSuite) cleanupTestData() {
	_, _ = ts.dbpool.Exec(ts.ctx, `DELETE FROM monthly_budgets WHERE budget_id = $1`, ts.budgetID)
}

func (ts *testSuite) createTableIfNotExists(t *testing.T) {
	_, err := ts.dbpool.Exec(ts.ctx, `
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
}

func (ts *testSuite) insertDummyData(t *testing.T) {
	ts.createTableIfNotExists(t)

	dummyRows := [][]any{
		{"2025-05", ts.budgetID, ts.catID, 1000.00, 0.00},     // activity: -1000
		{"2025-06", ts.budgetID, ts.catID, 2500.00, 0.00},     // activity: -2500
		{"2025-07", ts.budgetID, ts.catID, 2000.00, -5000.00}, // activity: -7000
		{"2025-08", ts.budgetID, ts.catID, 1000.00, 500.00},   // activity: 4500
	}

	for _, row := range dummyRows {
		_, err := ts.dbpool.Exec(ts.ctx, `
			INSERT INTO monthly_budgets (budget_id, category_id, month, budgeted, carryover_balance, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		`, row[1], row[2], row[0], row[3], row[4])
		require.NoError(t, err)
	}
}

// Test data retrieval utilities
func (ts *testSuite) getMonthlyBudgets(t *testing.T, fromMonth string) []model.MonthlyBudget {
	var monthlyBudgets []model.MonthlyBudget

	rows, err := ts.dbpool.Query(ts.ctx, `
		SELECT budgeted, carryover_balance, month 
		FROM monthly_budgets 
		WHERE budget_id = $1 AND category_id = $2 AND month >= $3
		ORDER BY month
	`, ts.budgetID, ts.catID, fromMonth)
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var mb model.MonthlyBudget
		err = rows.Scan(&mb.Budgeted, &mb.CarryoverBalance, &mb.Month)
		require.NoError(t, err)
		monthlyBudgets = append(monthlyBudgets, mb)
	}

	return monthlyBudgets
}

func (ts *testSuite) getSingleMonthlyBudget(t *testing.T, month string) (float64, float64) {
	var budgeted, carryover float64
	err := ts.dbpool.QueryRow(ts.ctx, `
		SELECT budgeted, carryover_balance 
		FROM monthly_budgets 
		WHERE budget_id = $1 AND category_id = $2 AND month = $3
	`, ts.budgetID, ts.catID, month).Scan(&budgeted, &carryover)
	require.NoError(t, err)
	return budgeted, carryover
}

// Test assertion utilities
func (ts *testSuite) assertMonthlyBudgets(t *testing.T, monthlyBudgets []model.MonthlyBudget, expected map[string]budgetData) {
	for _, mb := range monthlyBudgets {
		expectedData, ok := expected[mb.Month]
		require.True(t, ok, "Unexpected month: %s", mb.Month)
		require.Equal(t, expectedData.Budgeted, mb.Budgeted, "Budgeted amount mismatch for month %s", mb.Month)
		require.Equal(t, expectedData.CarryoverBalance, mb.CarryoverBalance, "Carryover balance mismatch for month %s", mb.Month)
	}
}

// Table-driven test helper functions
type createTestCase struct {
	name              string
	monthlyBudget     model.MonthlyBudget
	expectError       bool
	expectedBudget    float64
	expectedCarry     float64
	verifyAllMonths   bool
	expectedAllMonths map[string]budgetData
	description       string
}

type updateTestCase struct {
	name                string
	month               string
	newBudgeted         float64
	expectError         bool
	expectSpecificError bool
	expectedData        map[string]budgetData
	description         string
}

// TestCreateTableDriven demonstrates table-driven testing approach
func TestCreate(t *testing.T) {
	testCases := []createTestCase{
		{
			name: "FutureMonth",
			monthlyBudget: model.MonthlyBudget{
				Month:    "2025-09",
				Budgeted: 500.00,
			},
			expectError:    false,
			expectedBudget: 500.00,
			expectedCarry:  1000.00,
			description:    "Create budget for future month with carryover from previous",
		},
		{
			name: "PastMonthBeforeExistingData",
			monthlyBudget: model.MonthlyBudget{
				Month:    "2025-04",
				Budgeted: 500.00,
			},
			expectError:     false,
			expectedBudget:  500.00,
			expectedCarry:   500.00,
			description:     "Create budget for past month before existing data",
			verifyAllMonths: true,
			expectedAllMonths: map[string]budgetData{
				"2025-04": {Budgeted: 500.00, CarryoverBalance: 500.00},
				"2025-05": {Budgeted: 1000.00, CarryoverBalance: 500.00},
				"2025-06": {Budgeted: 2500.00, CarryoverBalance: 500.00},
				"2025-07": {Budgeted: 2000.00, CarryoverBalance: -4500.00},
				"2025-08": {Budgeted: 1000.00, CarryoverBalance: 1000.00},
			},
		},
		{
			name: "ExistingMonth",
			monthlyBudget: model.MonthlyBudget{
				Month:    "2025-06",
				Budgeted: 500.00,
			},
			expectError: true,
			description: "Attempt to create budget for existing month should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := setupTestSuite(t)
			defer ts.tearDown()
			defer ts.cleanupTestData()

			// Setup test data
			ts.insertDummyData(t)

			// Set IDs for the test case
			tc.monthlyBudget.BudgetID = ts.budgetID
			tc.monthlyBudget.CategoryID = ts.catID

			// Execute operation
			err := ts.repo.Create(ts.ctx, ts.budgetID, tc.monthlyBudget)

			if tc.expectError {
				require.Error(t, err, tc.description)
			} else {
				require.NoError(t, err, tc.description)
				// Verify results only if no error expected
				if tc.verifyAllMonths {
					// Verify all monthly budgets are updated correctly
					monthlyBudgets := ts.getMonthlyBudgets(t, tc.monthlyBudget.Month)
					ts.assertMonthlyBudgets(t, monthlyBudgets, tc.expectedAllMonths)
				} else {
					// Verify single budget record
					budgeted, carryover := ts.getSingleMonthlyBudget(t, tc.monthlyBudget.Month)
					require.Equal(t, tc.expectedBudget, budgeted)
					require.Equal(t, tc.expectedCarry, carryover)
				}
			}
		})
	}
}

// TestUpdateBudgetedByCatIdAndMonth demonstrates table-driven testing for updates
func TestUpdateBudgetedByCatIdAndMonth(t *testing.T) {
	testCases := []updateTestCase{
		{
			name:        "ValidUpdate",
			month:       "2025-07",
			newBudgeted: 5000.00,
			expectError: false,
			expectedData: map[string]budgetData{
				"2025-05": {Budgeted: 1000.00, CarryoverBalance: 0.00},
				"2025-06": {Budgeted: 2500.00, CarryoverBalance: 0.00},
				"2025-07": {Budgeted: 5000.00, CarryoverBalance: -2000.00},
				"2025-08": {Budgeted: 1000.00, CarryoverBalance: 3500.00},
			},
			description: "Update existing month budget",
		},
		{
			name:                "NonexistentMonth",
			month:               "2025-09",
			newBudgeted:         5000.00,
			expectError:         true,
			expectSpecificError: true,
			description:         "Update non-existent month should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := setupTestSuite(t)
			defer ts.tearDown()
			defer ts.cleanupTestData()

			// Setup test data
			ts.insertDummyData(t)

			// Execute operation
			err := ts.repo.UpdateBudgetedByCatIdAndMonth(ts.ctx, ts.budgetID, ts.catID, tc.month, tc.newBudgeted)

			if tc.expectError {
				require.Error(t, err, tc.description)
				if tc.expectSpecificError {
					require.ErrorIs(t, err, pgx.ErrNoRows, "Should return pgx.ErrNoRows for non-existent record")
				}
			} else {
				require.NoError(t, err, tc.description)
				// Verify results only if no error expected
				monthlyBudgets := ts.getMonthlyBudgets(t, "2025-05")
				ts.assertMonthlyBudgets(t, monthlyBudgets, tc.expectedData)
			}
		})
	}
}
