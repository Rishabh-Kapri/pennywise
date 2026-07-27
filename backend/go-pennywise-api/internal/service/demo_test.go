package service

import (
	"context"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ─────────────────────────────────────────────────────────────────────────────
// demo seed dataset
// ─────────────────────────────────────────────────────────────────────────────

func TestGenerateDemoTransactions(t *testing.T) {
	now := time.Now()
	txns := generateDemoTransactions(now)

	t.Run("is_not_empty", func(t *testing.T) {
		assert.NotEmpty(t, txns)
	})

	t.Run("has_no_future_dates", func(t *testing.T) {
		today := now.Format("2006-01-02")
		for _, txn := range txns {
			assert.LessOrEqual(t, txn.date.String(), today)
		}
	})

	t.Run("inflow_category_amounts_are_positive", func(t *testing.T) {
		// the transaction service rejects negative amounts on the inflow category
		for _, txn := range txns {
			if txn.category == demoInflowCategory {
				assert.Greater(t, txn.amount, 0.0, "txn %+v", txn)
			}
		}
	})

	t.Run("has_starting_balances", func(t *testing.T) {
		var starting []demoTxn
		for _, txn := range txns {
			if txn.payee == demoStartingBal {
				starting = append(starting, txn)
			}
		}
		assert.Len(t, starting, 3)
		for _, txn := range starting {
			if txn.account == demoCreditCardAccount {
				assert.Empty(t, txn.category, "credit card starting balance must have no category")
				assert.Less(t, txn.amount, 0.0)
			} else {
				assert.Equal(t, demoInflowCategory, txn.category)
				assert.Greater(t, txn.amount, 0.0)
			}
		}
	})

	t.Run("has_salary_every_month", func(t *testing.T) {
		count := 0
		for _, txn := range txns {
			if txn.payee == "Employer Inc." {
				count++
				assert.Equal(t, demoInflowCategory, txn.category)
			}
		}
		assert.Equal(t, demoSeedMonths+1, count)
	})

	t.Run("cc_payments_are_uncategorized_transfers", func(t *testing.T) {
		for _, txn := range txns {
			if txn.payee == demoCCTransferPayee {
				assert.Empty(t, txn.category, "budget transfers must have no category")
				assert.Equal(t, demoCheckingAccount, txn.account)
			}
		}
	})

	t.Run("is_deterministic", func(t *testing.T) {
		assert.Equal(t, txns, generateDemoTransactions(now))
	})
}

func TestGenerateDemoTransactions_ReferencesSeededNames(t *testing.T) {
	payees := map[string]bool{demoStartingBal: true, demoCCTransferPayee: true}
	for _, name := range demoPayeeNames() {
		payees[name] = true
	}
	categories := map[string]bool{demoInflowCategory: true}
	for _, group := range demoTemplateGroups() {
		for _, cat := range group.Categories {
			categories[cat.Name] = true
		}
	}
	accounts := map[string]bool{}
	for _, acc := range demoAccounts() {
		accounts[acc.Name] = true
	}

	for _, txn := range generateDemoTransactions(time.Now()) {
		assert.True(t, payees[txn.payee], "unknown payee %q", txn.payee)
		assert.True(t, accounts[txn.account], "unknown account %q", txn.account)
		if txn.category != "" {
			assert.True(t, categories[txn.category], "unknown category %q", txn.category)
		}
	}
}

func TestDemoMonthlyBudgeted_ReferencesSeededCategories(t *testing.T) {
	categories := map[string]bool{}
	for _, group := range demoTemplateGroups() {
		for _, cat := range group.Categories {
			categories[cat.Name] = true
		}
	}
	for name := range demoMonthlyBudgeted() {
		assert.True(t, categories[name], "budgeted category %q is not seeded", name)
	}
}

func demoTestIDMaps() (payeeIDs, accountIDs, categoryIDs map[string]uuid.UUID) {
	payeeIDs = map[string]uuid.UUID{demoStartingBal: uuid.New()}
	for _, name := range demoPayeeNames() {
		payeeIDs[name] = uuid.New()
	}
	accountIDs = map[string]uuid.UUID{}
	for _, acc := range demoAccounts() {
		accountIDs[acc.Name] = uuid.New()
		payeeIDs["Transfer : "+acc.Name] = uuid.New()
	}
	categoryIDs = map[string]uuid.UUID{demoInflowCategory: uuid.New()}
	for _, group := range demoTemplateGroups() {
		for _, cat := range group.Categories {
			categoryIDs[cat.Name] = uuid.New()
		}
	}
	return payeeIDs, accountIDs, categoryIDs
}

func TestBuildDemoSeedRows(t *testing.T) {
	now := time.Now()
	payeeIDs, accountIDs, categoryIDs := demoTestIDMaps()

	rows, carryover, err := buildDemoSeedRows(now, payeeIDs, accountIDs, categoryIDs)
	assert.NoError(t, err)
	assert.NotEmpty(t, rows)

	t.Run("cc_payments_have_linked_counterparts", func(t *testing.T) {
		byID := map[uuid.UUID]seedTxnRow{}
		for _, r := range rows {
			byID[r.id] = r
		}
		transfers := 0
		for _, r := range rows {
			if r.transferTransactionID == nil {
				continue
			}
			transfers++
			other, ok := byID[*r.transferTransactionID]
			assert.True(t, ok, "counterpart of %s missing", r.id)
			assert.Equal(t, -r.amount, other.amount)
			assert.Equal(t, r.id, *other.transferTransactionID, "back-link broken")
			assert.Equal(t, r.accountID, *other.transferAccountID)
			assert.Nil(t, r.categoryID, "transfers must have no category")
		}
		assert.NotZero(t, transfers)
		assert.Equal(t, 0, transfers%2, "transfers must come in pairs")
	})

	t.Run("carryover_matches_categorized_spend", func(t *testing.T) {
		idToName := map[uuid.UUID]string{}
		for name, id := range categoryIDs {
			idToName[id] = name
		}
		expected := map[string]map[string]float64{}
		for _, r := range rows {
			if r.categoryID == nil {
				continue
			}
			name := idToName[*r.categoryID]
			if name == demoInflowCategory {
				continue
			}
			monthKey := r.date[:7]
			if expected[monthKey] == nil {
				expected[monthKey] = map[string]float64{}
			}
			expected[monthKey][name] += r.amount
		}
		assert.Equal(t, expected, carryover)
	})
}

func TestBuildDemoMonthlyRows(t *testing.T) {
	now := time.Now()
	payeeIDs, accountIDs, categoryIDs := demoTestIDMaps()
	_, carryover, err := buildDemoSeedRows(now, payeeIDs, accountIDs, categoryIDs)
	assert.NoError(t, err)

	rows, err := buildDemoMonthlyRows(now, categoryIDs, carryover)
	assert.NoError(t, err)
	// one row per budgeted category per seeded month (all activity
	// categories are budgeted, so no extra carryover-only rows)
	assert.Len(t, rows, (demoSeedMonths+1)*len(demoMonthlyBudgeted()))

	// the oldest month is always fully populated regardless of today's date
	rentID := categoryIDs["Rent"]
	oldestMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).
		AddDate(0, -demoSeedMonths, 0).
		Format("2006-01")
	found := false
	for _, r := range rows {
		if r.categoryID == rentID && r.month == oldestMonth {
			found = true
			assert.Equal(t, 18000.0, r.budgeted)
			assert.Equal(t, -18000.0, r.carryover, "rent activity should carry over")
		}
	}
	assert.True(t, found)
}

func TestDemoPayeeRules_ReferenceSeededNames(t *testing.T) {
	payees := map[string]bool{}
	for _, name := range demoPayeeNames() {
		payees[name] = true
	}
	categories := map[string]bool{}
	for _, group := range demoTemplateGroups() {
		for _, cat := range group.Categories {
			categories[cat.Name] = true
		}
	}
	seen := map[string]bool{}
	for _, rule := range demoPayeeRules() {
		assert.True(t, payees[rule.payee], "rule payee %q is not seeded", rule.payee)
		assert.True(t, categories[rule.category], "rule category %q is not seeded", rule.category)
		assert.Contains(t, []string{"EXACT", "PATTERN"}, rule.matchType)
		assert.False(t, seen[rule.matchString], "duplicate match string %q violates unique constraint", rule.matchString)
		seen[rule.matchString] = true
	}
}

func TestBuildDemoPredictionRows(t *testing.T) {
	now := time.Now()
	payeeIDs, accountIDs, categoryIDs := demoTestIDMaps()
	txnRows, _, err := buildDemoSeedRows(now, payeeIDs, accountIDs, categoryIDs)
	assert.NoError(t, err)

	rows, err := buildDemoPredictionRows(txnRows, payeeIDs, accountIDs, categoryIDs)
	assert.NoError(t, err)
	assert.NotEmpty(t, rows)

	txnByID := map[uuid.UUID]seedTxnRow{}
	for _, r := range txnRows {
		txnByID[r.id] = r
	}
	catByID := map[uuid.UUID]string{}
	for name, id := range categoryIDs {
		catByID[id] = name
	}

	sources := map[model.PredictionSource]int{}
	corrected := 0
	for _, row := range rows {
		txn, ok := txnByID[row.txnID]
		assert.True(t, ok, "prediction references unknown transaction")
		assert.Equal(t, -txn.amount, row.amount, "prediction amount is the debited (positive) amount")
		assert.NotEmpty(t, row.emailText)
		assert.NotEmpty(t, row.extractedAccount)
		sources[row.source]++

		switch row.source {
		case model.PredictionSourceUncategorized:
			assert.Nil(t, row.predictedCategoryID)
			assert.True(t, row.hasUserCorrected)
			assert.NotNil(t, row.actualCategoryID)
		default:
			assert.Equal(t, txn.payeeID, *row.predictedPayeeID, "prediction must point at the txn's payee")
			assert.NotNil(t, row.predictedCategoryID)
		}
		if row.source == model.PredictionSourceLLM {
			assert.NotNil(t, row.reasoning)
			assert.NotEmpty(t, row.metadata)
		}
		if row.hasUserCorrected && row.source != model.PredictionSourceUncategorized {
			corrected++
			assert.NotNil(t, row.actualCategoryID)
			assert.NotEqual(t, *row.predictedCategoryID, *row.actualCategoryID,
				"a corrected prediction must have predicted a different category")
			// the user's correction matches how the transaction is actually categorized
			assert.Equal(t, *txn.categoryID, *row.actualCategoryID)
		}
	}
	for _, source := range []model.PredictionSource{
		model.PredictionSourceRule, model.PredictionSourceVector,
		model.PredictionSourceLLM, model.PredictionSourceUncategorized,
	} {
		assert.NotZero(t, sources[source], "expected at least one %s prediction", source)
	}
	assert.Equal(t, len(demoPredictionCorrections()), corrected,
		"exactly one correction per configured payee")

	t.Run("uncorrected_predictions_match_txn_category", func(t *testing.T) {
		for _, row := range rows {
			if row.hasUserCorrected {
				continue
			}
			txn := txnByID[row.txnID]
			assert.Equal(t, *txn.categoryID, *row.predictedCategoryID,
				"payee %v: an accepted prediction should match the transaction's category",
				catByID[*txn.categoryID])
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DemoService — existing demo user path (seeding is covered end-to-end)
// ─────────────────────────────────────────────────────────────────────────────

func TestDemoService_LoginAsDemo_ExistingUser(t *testing.T) {
	authUser := &model.AuthUser{ID: uuid.New(), TokenVersion: 2}
	creds := &model.UserWithCredentials{
		AuthUser: authUser,
		GoogleProvider: &model.GoogleProviderUser{
			ID:    demoGoogleID,
			Email: demoUserEmail,
			Name:  demoUserName,
		},
	}

	googleRepo := &svcGoogleProviderRepo{}
	googleRepo.On("GetUserByGoogleIDAndClientType", mock.Anything, demoGoogleID, model.GoogleOAuthClientTypeWeb).
		Return(creds, nil)
	authRepo := &svcAuthRepo{}
	authRepo.On("SaveRefreshTokenHash", mock.Anything, authUser.ID, mock.Anything).Return(nil)

	// seeding dependencies are nil: an existing user must never touch them
	svc := NewDemoService(
		newAuthSvcForTest(authRepo),
		authRepo,
		googleRepo,
		nil, nil, nil, nil, nil, nil,
	)

	user, accessToken, refreshToken, err := svc.LoginAsDemo(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, authUser.ID, user.ID)
	assert.Equal(t, demoUserEmail, user.Email)
	assert.Equal(t, demoUserName, user.Name)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	googleRepo.AssertExpectations(t)
	authRepo.AssertExpectations(t)
}
