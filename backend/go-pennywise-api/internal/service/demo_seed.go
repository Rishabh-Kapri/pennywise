package service

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
)

const (
	demoGoogleID   = "pennywise-demo-user"
	demoUserEmail  = "demo@pennywise.local"
	demoUserName   = "Demo User"
	demoBudgetName = "Demo Budget"

	demoInflowCategory = "Inflow: Ready to Assign"
	demoStartingBal    = "Starting Balance"

	demoCheckingAccount   = "HDFC Checking"
	demoSavingsAccount    = "SBI Savings"
	demoCreditCardAccount = "ICICI Credit Card"

	demoCCTransferPayee = "Transfer : " + demoCreditCardAccount

	// number of past months to seed in addition to the current month
	demoSeedMonths = 3
)

func demoTemplateGroups() []model.BudgetTemplateGroup {
	return []model.BudgetTemplateGroup{
		{Name: "Bills", Categories: []model.BudgetTemplateCategory{
			{Name: "Rent"}, {Name: "Electricity"}, {Name: "Internet"}, {Name: "Phone"},
		}},
		{Name: "Food", Categories: []model.BudgetTemplateCategory{
			{Name: "Groceries"}, {Name: "Dining Out"}, {Name: "Coffee"},
		}},
		{Name: "Transport", Categories: []model.BudgetTemplateCategory{
			{Name: "Fuel"}, {Name: "Cab & Metro"},
		}},
		{Name: "Lifestyle", Categories: []model.BudgetTemplateCategory{
			{Name: "Shopping"}, {Name: "Entertainment"}, {Name: "Subscriptions"},
		}},
		{Name: "Health", Categories: []model.BudgetTemplateCategory{
			{Name: "Gym"}, {Name: "Medical"},
		}},
		{Name: "Savings Goals", Categories: []model.BudgetTemplateCategory{
			{Name: "Emergency Fund"}, {Name: "Vacation"},
		}},
	}
}

func demoAccounts() []model.Account {
	return []model.Account{
		{Name: demoCheckingAccount, Type: "checking"},
		{Name: demoSavingsAccount, Type: "savings"},
		{Name: demoCreditCardAccount, Type: "creditCard"},
	}
}

func demoPayeeNames() []string {
	return []string{
		"Employer Inc.", "Landlord", "BigBasket", "Swiggy", "Zomato", "Amazon",
		"Netflix", "Uber", "Indian Oil", "Blue Tokai Coffee", "Apollo Pharmacy",
		"Cult Fitness", "Airtel", "Tata Power",
	}
}

// demoMonthlyBudgeted totals ~56,700 against a 75,000 salary so Ready to
// Assign stays positive every month.
func demoMonthlyBudgeted() map[string]float64 {
	return map[string]float64{
		"Rent":           18000,
		"Electricity":    1500,
		"Internet":       1000,
		"Phone":          500,
		"Groceries":      9000,
		"Dining Out":     4000,
		"Coffee":         1500,
		"Fuel":           2500,
		"Cab & Metro":    1500,
		"Shopping":       5000,
		"Entertainment":  1000,
		"Subscriptions":  700,
		"Gym":            1500,
		"Medical":        1000,
		"Emergency Fund": 5000,
		"Vacation":       3000,
	}
}

// demoTxn is a transaction spec with names instead of IDs; category == ""
// means no category (budget transfers and the credit card starting balance).
type demoTxn struct {
	date     model.Date
	payee    string
	category string
	account  string
	amount   float64
}

// generateDemoTransactions produces ~4 months of transactions relative to
// now so the seeded data never goes stale. The RNG is fixed-seeded so the
// dataset is deterministic for a given day.
func generateDemoTransactions(now time.Time) []demoTxn {
	rng := rand.New(rand.NewSource(42))
	var txns []demoTxn

	monthStart := func(offset int) time.Time {
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -offset, 0)
	}
	add := func(month time.Time, day int, payee, category, account string, amount float64) {
		date := time.Date(month.Year(), month.Month(), day, 0, 0, 0, 0, time.UTC)
		if date.After(now) {
			return
		}
		txns = append(txns, demoTxn{
			date:     model.Date(date.Format("2006-01-02")),
			payee:    payee,
			category: category,
			account:  account,
			amount:   amount,
		})
	}
	spend := func(min, max float64) float64 {
		return -math.Round(min + rng.Float64()*(max-min))
	}

	// starting balances in the oldest month; the credit card balance is
	// negative so it must not use the inflow category
	oldest := monthStart(demoSeedMonths)
	add(oldest, 1, demoStartingBal, demoInflowCategory, demoCheckingAccount, 50000)
	add(oldest, 1, demoStartingBal, demoInflowCategory, demoSavingsAccount, 150000)
	add(oldest, 1, demoStartingBal, "", demoCreditCardAccount, -12000)

	for offset := demoSeedMonths; offset >= 0; offset-- {
		month := monthStart(offset)

		add(month, 1, "Employer Inc.", demoInflowCategory, demoCheckingAccount, 75000)

		add(month, 2, "Landlord", "Rent", demoCheckingAccount, -18000)
		add(month, 8, "Tata Power", "Electricity", demoCheckingAccount, spend(800, 1400))
		add(month, 5, "Airtel", "Internet", demoCheckingAccount, -999)
		add(month, 5, "Airtel", "Phone", demoCheckingAccount, -499)
		add(month, 15, "Netflix", "Subscriptions", demoCheckingAccount, -649)
		add(month, 3, "Cult Fitness", "Gym", demoCheckingAccount, -1500)

		for _, day := range []int{4, 11, 18, 25} {
			add(month, day, "BigBasket", "Groceries", demoCheckingAccount, spend(1500, 2600))
		}
		for i := 0; i < 5; i++ {
			payee := "Swiggy"
			account := demoCheckingAccount
			if i%2 == 0 {
				payee = "Zomato"
				account = demoCreditCardAccount
			}
			add(month, 2+rng.Intn(26), payee, "Dining Out", account, spend(250, 900))
		}
		for i := 0; i < 6; i++ {
			add(month, 2+rng.Intn(26), "Blue Tokai Coffee", "Coffee", demoCheckingAccount, spend(180, 350))
		}
		for _, day := range []int{6, 20} {
			add(month, day, "Indian Oil", "Fuel", demoCheckingAccount, spend(1000, 1400))
		}
		for i := 0; i < 4; i++ {
			add(month, 2+rng.Intn(26), "Uber", "Cab & Metro", demoCheckingAccount, spend(120, 450))
		}
		for i := 0; i < 1+rng.Intn(2); i++ {
			add(month, 5+rng.Intn(20), "Amazon", "Shopping", demoCreditCardAccount, spend(800, 4500))
		}
		if rng.Intn(2) == 0 {
			add(month, 12+rng.Intn(10), "Apollo Pharmacy", "Medical", demoCheckingAccount, spend(300, 1200))
		}

		// monthly credit card payment from checking to the card
		add(month, 28, demoCCTransferPayee, "", demoCheckingAccount, -8000)
	}

	return txns
}

// demoPayeeRule maps a UPI handle / bank narration string to a payee and
// category, showcasing the rule-based half of transaction classification.
type demoPayeeRule struct {
	matchString string
	matchType   string // EXACT or PATTERN
	payee       string
	category    string
}

func demoPayeeRules() []demoPayeeRule {
	return []demoPayeeRule{
		{"swiggy@icici", "EXACT", "Swiggy", "Dining Out"},
		{"zomato-order@paytm", "EXACT", "Zomato", "Dining Out"},
		{"netflix@hdfcbank", "EXACT", "Netflix", "Subscriptions"},
		{"cult.fit@hdfcbank", "EXACT", "Cult Fitness", "Gym"},
		{"airtel@payu", "EXACT", "Airtel", "Internet"},
		{"%bigbasket%", "PATTERN", "BigBasket", "Groceries"},
		{"%blue tokai%", "PATTERN", "Blue Tokai Coffee", "Coffee"},
		{"%uber india%", "PATTERN", "Uber", "Cab & Metro"},
	}
}

// demoPredictionSpec describes how the classifier "saw" transactions of a
// given payee: which pipeline stage matched (rule/vector/LLM), what was
// extracted from the bank email, and how confident it was.
type demoPredictionSpec struct {
	source    model.PredictionSource
	merchant  string // extracted merchant string from the email
	handle    string // UPI handle / narration in the email text
	category  string // predicted category name
	payeeConf float64
	catConf   float64
	reasoning string // LLM only
}

func demoPredictionSpecs() map[string]demoPredictionSpec {
	return map[string]demoPredictionSpec{
		// matched by the seeded payee rules → RULE, full confidence
		"Swiggy":       {source: model.PredictionSourceRule, merchant: "SWIGGY", handle: "swiggy@icici", category: "Dining Out", payeeConf: 1, catConf: 1},
		"Zomato":       {source: model.PredictionSourceRule, merchant: "ZOMATO LTD", handle: "zomato-order@paytm", category: "Dining Out", payeeConf: 1, catConf: 1},
		"Netflix":      {source: model.PredictionSourceRule, merchant: "NETFLIX ENTERTAINMENT", handle: "netflix@hdfcbank", category: "Subscriptions", payeeConf: 1, catConf: 1},
		"Cult Fitness": {source: model.PredictionSourceRule, merchant: "CULTFIT HEALTHCARE", handle: "cult.fit@hdfcbank", category: "Gym", payeeConf: 1, catConf: 1},
		// matched by embedding similarity to past transactions → VECTOR
		"BigBasket":         {source: model.PredictionSourceVector, merchant: "BIGBASKET BANGALORE", handle: "bigbasket.supermart@ybl", category: "Groceries", payeeConf: 0.93, catConf: 0.91},
		"Blue Tokai Coffee": {source: model.PredictionSourceVector, merchant: "BLUE TOKAI COFFEE ROASTERS", handle: "blue tokai coffee@icici", category: "Coffee", payeeConf: 0.89, catConf: 0.87},
		"Uber":              {source: model.PredictionSourceVector, merchant: "UBER INDIA SYSTEMS", handle: "uber india rides@paytm", category: "Cab & Metro", payeeConf: 0.9, catConf: 0.86},
		// no rule or vector match → LLM classification with reasoning
		"Amazon": {
			source: model.PredictionSourceLLM, merchant: "AMAZON PAY INDIA", handle: "amazonpay@apl", category: "Shopping",
			payeeConf: 0.84, catConf: 0.78,
			reasoning: "The narration 'AMAZON PAY INDIA' maps to the existing payee 'Amazon'. Past Amazon transactions in this budget are categorized as Shopping, and the amount is consistent with a retail purchase rather than a subscription.",
		},
		"Apollo Pharmacy": {
			source: model.PredictionSourceLLM, merchant: "APOLLO PHARMACY", handle: "apollopharmacy@ybl", category: "Medical",
			payeeConf: 0.88, catConf: 0.82,
			reasoning: "The merchant 'APOLLO PHARMACY' is a pharmacy chain, so the transaction is classified under Medical. The payee 'Apollo Pharmacy' already exists in this budget.",
		},
		"Indian Oil": {
			source: model.PredictionSourceLLM, merchant: "IOCL PETROL PUMP", handle: "iocl.fuel@sbi", category: "Fuel",
			payeeConf: 0.8, catConf: 0.75,
			reasoning: "The narration 'IOCL PETROL PUMP' refers to an Indian Oil fuel station. The amount matches typical fuel spends for this budget, so it is categorized as Fuel.",
		},
		// classifier could not decide → UNCATEGORIZED, user fixed it manually
		"Tata Power": {source: model.PredictionSourceUncategorized, merchant: "TATA POWER MUMBAI", handle: "tatapower.bill@payu", payeeConf: 0.42},
	}
}

// demoPredictionCorrections marks the first prediction of a payee as
// user-corrected: the classifier predicted the wrong category and the user
// fixed it, which feeds the accuracy stats on the AI settings page.
func demoPredictionCorrections() map[string]string {
	return map[string]string{
		"Amazon":     "Entertainment", // predicted Entertainment, user corrected to Shopping
		"Indian Oil": "Cab & Metro",   // predicted Cab & Metro, user corrected to Fuel
	}
}

// seedPredictionRow is a cipher_predictions row ready for bulk COPY.
type seedPredictionRow struct {
	txnID               uuid.UUID
	emailText           string
	reasoning           *string
	metadata            json.RawMessage
	amount              float64
	extractedAccount    string
	extractedMerchant   string
	predictedPayeeID    *uuid.UUID
	predictedCategoryID *uuid.UUID
	payeeConf           *float64
	catConf             *float64
	source              model.PredictionSource
	hasUserCorrected    bool
	actualPayeeID       *uuid.UUID
	actualCategoryID    *uuid.UUID
	createdAt           time.Time
}

// buildDemoPredictionRows attaches a cipher prediction to every seeded
// transaction whose payee has a prediction spec, simulating what the email
// parsing pipeline would have produced.
func buildDemoPredictionRows(
	txnRows []seedTxnRow,
	payeeIDs, accountIDs, categoryIDs map[string]uuid.UUID,
) ([]seedPredictionRow, error) {
	payeeNames := map[uuid.UUID]string{}
	for name, id := range payeeIDs {
		payeeNames[id] = name
	}
	accountNames := map[uuid.UUID]string{}
	for name, id := range accountIDs {
		accountNames[id] = name
	}
	maskedAccounts := map[string]string{
		demoCheckingAccount:   "HDFC Bank XX4523",
		demoSavingsAccount:    "SBI XX9034",
		demoCreditCardAccount: "ICICI Credit Card XX8811",
	}
	specs := demoPredictionSpecs()
	corrections := demoPredictionCorrections()

	rng := rand.New(rand.NewSource(7))
	corrected := map[string]bool{}
	var rows []seedPredictionRow
	for _, txn := range txnRows {
		payeeName := payeeNames[txn.payeeID]
		spec, ok := specs[payeeName]
		if !ok {
			continue
		}
		payeeID := txn.payeeID
		date, err := time.Parse("2006-01-02", txn.date)
		if err != nil {
			return nil, fmt.Errorf("demo seed; invalid txn date %q: %w", txn.date, err)
		}

		row := seedPredictionRow{
			txnID:             txn.id,
			amount:            -txn.amount,
			extractedAccount:  maskedAccounts[accountNames[txn.accountID]],
			extractedMerchant: spec.merchant,
			source:            spec.source,
			createdAt:         date.Add(time.Duration(9+rng.Intn(10)) * time.Hour),
			emailText: fmt.Sprintf(
				"Dear Customer, INR %.2f has been debited from %s for a payment to VPA %s (%s) on %s. Ref No %09d. If this was not you, please call your bank immediately.",
				-txn.amount, maskedAccounts[accountNames[txn.accountID]], spec.handle, spec.merchant,
				date.Format("02-01-06"), rng.Intn(1_000_000_000),
			),
		}
		if spec.payeeConf > 0 {
			conf := spec.payeeConf
			row.payeeConf = &conf
		}
		if spec.reasoning != "" {
			reasoning := spec.reasoning
			row.reasoning = &reasoning
			row.metadata = json.RawMessage(fmt.Sprintf(
				`{"provider":"openrouter","model":"anthropic/claude-haiku-4.5","latencyMs":%d}`,
				800+rng.Intn(1200),
			))
		}

		switch spec.source {
		case model.PredictionSourceUncategorized:
			// classifier gave up: no predictions, user categorized it manually
			row.hasUserCorrected = true
			row.actualPayeeID = &payeeID
			actualCat, ok := categoryIDs["Electricity"]
			if !ok {
				return nil, fmt.Errorf("demo seed; category %q not found", "Electricity")
			}
			row.actualCategoryID = &actualCat
		default:
			row.predictedPayeeID = &payeeID
			predictedName := spec.category
			if wrong, hasCorrection := corrections[payeeName]; hasCorrection && !corrected[payeeName] {
				predictedName = wrong
			}
			predictedCat, ok := categoryIDs[predictedName]
			if !ok {
				return nil, fmt.Errorf("demo seed; category %q not found", predictedName)
			}
			row.predictedCategoryID = &predictedCat
			if spec.catConf > 0 {
				conf := spec.catConf
				row.catConf = &conf
			}
			if predictedName != spec.category {
				corrected[payeeName] = true
				row.hasUserCorrected = true
				row.actualPayeeID = &payeeID
				actualCat, ok := categoryIDs[spec.category]
				if !ok {
					return nil, fmt.Errorf("demo seed; category %q not found", spec.category)
				}
				row.actualCategoryID = &actualCat
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// seedTxnRow is a fully resolved transactions row ready for bulk COPY.
type seedTxnRow struct {
	id                    uuid.UUID
	date                  string
	payeeID               uuid.UUID
	categoryID            *uuid.UUID
	accountID             uuid.UUID
	amount                float64
	transferAccountID     *uuid.UUID
	transferTransactionID *uuid.UUID
}

// seedMonthlyRow is a monthly_budgets row ready for bulk COPY.
type seedMonthlyRow struct {
	month      string
	categoryID uuid.UUID
	budgeted   float64
	carryover  float64
}

// buildDemoSeedRows resolves the generated specs into insertable rows,
// expanding the credit card payments into linked transfer pairs (what
// transactionService.applySideEffects would do) and accumulating the
// per-category carryover deltas that UpsertCarryover would apply.
func buildDemoSeedRows(
	now time.Time,
	payeeIDs, accountIDs, categoryIDs map[string]uuid.UUID,
) ([]seedTxnRow, map[string]map[string]float64, error) {
	var rows []seedTxnRow
	carryover := map[string]map[string]float64{}

	for _, spec := range generateDemoTransactions(now) {
		payeeID, ok := payeeIDs[spec.payee]
		if !ok {
			return nil, nil, fmt.Errorf("demo seed; payee %q not found", spec.payee)
		}
		accountID, ok := accountIDs[spec.account]
		if !ok {
			return nil, nil, fmt.Errorf("demo seed; account %q not found", spec.account)
		}
		row := seedTxnRow{
			id:        uuid.New(),
			date:      spec.date.String(),
			payeeID:   payeeID,
			accountID: accountID,
			amount:    spec.amount,
		}
		if spec.category != "" {
			categoryID, ok := categoryIDs[spec.category]
			if !ok {
				return nil, nil, fmt.Errorf("demo seed; category %q not found", spec.category)
			}
			row.categoryID = &categoryID
			if spec.category != demoInflowCategory {
				monthKey := spec.date.String()[:7] // YYYY-MM
				if carryover[monthKey] == nil {
					carryover[monthKey] = map[string]float64{}
				}
				carryover[monthKey][spec.category] += spec.amount
			}
		}

		if spec.payee == demoCCTransferPayee {
			// budget transfer: counterpart inflow on the card, both sides linked
			ccAccountID, ok := accountIDs[demoCreditCardAccount]
			if !ok {
				return nil, nil, fmt.Errorf("demo seed; account %q not found", demoCreditCardAccount)
			}
			sourcePayeeID, ok := payeeIDs["Transfer : "+spec.account]
			if !ok {
				return nil, nil, fmt.Errorf("demo seed; payee %q not found", "Transfer : "+spec.account)
			}
			counterpart := seedTxnRow{
				id:                    uuid.New(),
				date:                  row.date,
				payeeID:               sourcePayeeID,
				accountID:             ccAccountID,
				amount:                -spec.amount,
				transferAccountID:     &row.accountID,
				transferTransactionID: &row.id,
			}
			row.transferAccountID = &ccAccountID
			row.transferTransactionID = &counterpart.id
			rows = append(rows, row, counterpart)
			continue
		}
		rows = append(rows, row)
	}
	return rows, carryover, nil
}

// buildDemoMonthlyRows produces the monthly_budgets rows: the fixed budgeted
// amounts plus the carryover balances accumulated from the transactions.
func buildDemoMonthlyRows(
	now time.Time,
	categoryIDs map[string]uuid.UUID,
	carryover map[string]map[string]float64,
) ([]seedMonthlyRow, error) {
	var rows []seedMonthlyRow
	for offset := demoSeedMonths; offset >= 0; offset-- {
		monthKey := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).
			AddDate(0, -offset, 0).
			Format("2006-01")
		seen := map[string]bool{}
		for name, budgeted := range demoMonthlyBudgeted() {
			categoryID, ok := categoryIDs[name]
			if !ok {
				return nil, fmt.Errorf("demo seed; budgeted category %q not found", name)
			}
			rows = append(rows, seedMonthlyRow{
				month:      monthKey,
				categoryID: categoryID,
				budgeted:   budgeted,
				carryover:  carryover[monthKey][name],
			})
			seen[name] = true
		}
		// categories with activity but no budgeted amount still need a row
		for name, delta := range carryover[monthKey] {
			if seen[name] {
				continue
			}
			categoryID, ok := categoryIDs[name]
			if !ok {
				return nil, fmt.Errorf("demo seed; carryover category %q not found", name)
			}
			rows = append(rows, seedMonthlyRow{month: monthKey, categoryID: categoryID, carryover: delta})
		}
	}
	return rows, nil
}
