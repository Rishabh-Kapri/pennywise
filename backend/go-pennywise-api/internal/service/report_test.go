package service

import (
	"math"
	"reflect"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
)

func TestAddMonthsToKey(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		delta int
		want  string
	}{
		{"forward within year", "2026-03", 2, "2026-05"},
		{"backward within year", "2026-07", -3, "2026-04"},
		{"december rollover forward", "2025-12", 1, "2026-01"},
		{"january rollover backward", "2026-01", -1, "2025-12"},
		{"multi-year forward", "2024-11", 14, "2026-01"},
		{"zero delta", "2026-07", 0, "2026-07"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addMonthsToKey(tt.key, tt.delta); got != tt.want {
				t.Errorf("addMonthsToKey(%q, %d) = %q, want %q", tt.key, tt.delta, got, tt.want)
			}
		})
	}
}

func TestMonthBounds(t *testing.T) {
	tests := []struct {
		name        string
		startMonth  string
		endMonth    string
		wantStart   string
		wantEndExcl string
	}{
		{"same month", "2026-07", "2026-07", "2026-07-01", "2026-08-01"},
		{"range", "2026-01", "2026-06", "2026-01-01", "2026-07-01"},
		{"december end rolls to january", "2025-10", "2025-12", "2025-10-01", "2026-01-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := monthBounds(tt.startMonth, tt.endMonth)
			if gotStart != tt.wantStart || gotEnd != tt.wantEndExcl {
				t.Errorf("monthBounds(%q, %q) = (%q, %q), want (%q, %q)",
					tt.startMonth, tt.endMonth, gotStart, gotEnd, tt.wantStart, tt.wantEndExcl)
			}
		})
	}
}

func TestMonthRange(t *testing.T) {
	tests := []struct {
		name  string
		start string
		end   string
		want  []string
	}{
		{"single month", "2026-07", "2026-07", []string{"2026-07"}},
		{"year rollover", "2025-11", "2026-02", []string{"2025-11", "2025-12", "2026-01", "2026-02"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := monthRange(tt.start, tt.end); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("monthRange(%q, %q) = %v, want %v", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestNormalizeMonthRange(t *testing.T) {
	t.Run("inverted range rejected", func(t *testing.T) {
		if _, _, err := normalizeMonthRange("2026-08", "2026-07"); err != ErrInvalidReportRange {
			t.Errorf("expected ErrInvalidReportRange, got %v", err)
		}
	})
	t.Run("explicit range kept", func(t *testing.T) {
		start, end, err := normalizeMonthRange("2026-01", "2026-07")
		if err != nil || start != "2026-01" || end != "2026-07" {
			t.Errorf("got (%q, %q, %v)", start, end, err)
		}
	})
	t.Run("defaults produce a valid window", func(t *testing.T) {
		start, end, err := normalizeMonthRange("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := monthRange(start, end); len(got) != defaultReportMonths {
			t.Errorf("default window is %d months (%q..%q), want %d", len(got), start, end, defaultReportMonths)
		}
	})
	t.Run("default start from explicit end", func(t *testing.T) {
		start, end, err := normalizeMonthRange("", "2026-03")
		if err != nil || end != "2026-03" || start != "2025-10" {
			t.Errorf("got (%q, %q, %v), want (2025-10, 2026-03)", start, end, err)
		}
	})
}

func TestAssembleSpendingReport(t *testing.T) {
	groceriesGroup := uuid.New()
	funGroup := uuid.New()
	catA := uuid.New()
	catB := uuid.New()
	catC := uuid.New()

	rows := []model.SpendingRow{
		{GroupID: groceriesGroup, GroupName: "Essentials", CategoryID: catA, CategoryName: "Groceries", Net: -300},
		{GroupID: funGroup, GroupName: "Fun", CategoryID: catB, CategoryName: "Eating Out", Net: -500},
		{GroupID: groceriesGroup, GroupName: "Essentials", CategoryID: catC, CategoryName: "Utilities", Net: 0},
	}

	report := assembleSpendingReport(rows)

	if report.Total != 800 {
		t.Errorf("Total = %v, want 800", report.Total)
	}
	if len(report.Groups) != 2 {
		t.Fatalf("got %d groups, want 2 (zero rows dropped)", len(report.Groups))
	}
	if report.Groups[0].Name != "Fun" || report.Groups[0].Total != 500 {
		t.Errorf("groups not sorted by total desc: first = %+v", report.Groups[0])
	}
	if len(report.Groups[1].Categories) != 1 || report.Groups[1].Categories[0].Total != 300 {
		t.Errorf("Essentials categories wrong: %+v", report.Groups[1].Categories)
	}
}

func TestAssembleIncomeExpenseReport(t *testing.T) {
	payee := uuid.New()
	group := uuid.New()
	cat := uuid.New()
	months := []string{"2026-06", "2026-07"}

	incomeRows := []model.IncomeRow{
		{Month: "2026-06", PayeeID: payee, PayeeName: "Employer", Amount: 1000},
	}
	expenseRows := []model.ExpenseRow{
		{Month: "2026-06", GroupID: group, GroupName: "Essentials", CategoryID: cat, CategoryName: "Groceries", Net: -400},
		{Month: "2026-07", GroupID: group, GroupName: "Essentials", CategoryID: cat, CategoryName: "Groceries", Net: -100},
	}

	report := assembleIncomeExpenseReport(months, incomeRows, expenseRows)

	if report.Income.Totals["2026-06"] != 1000 || report.Income.Totals["2026-07"] != 0 {
		t.Errorf("income totals wrong: %v", report.Income.Totals)
	}
	if report.Expense.Totals["2026-06"] != 400 || report.Expense.Totals["2026-07"] != 100 {
		t.Errorf("expense totals wrong (should be positive): %v", report.Expense.Totals)
	}
	if report.Net["2026-06"] != 600 || report.Net["2026-07"] != -100 {
		t.Errorf("net wrong: %v", report.Net)
	}
	if len(report.Expense.Groups) != 1 || len(report.Expense.Groups[0].Categories) != 1 {
		t.Fatalf("expense matrix shape wrong: %+v", report.Expense.Groups)
	}
	amounts := report.Expense.Groups[0].Categories[0].Amounts
	if amounts["2026-06"] != 400 || amounts["2026-07"] != 100 {
		t.Errorf("category amounts wrong: %v", amounts)
	}
	if len(report.Income.Payees) != 1 || report.Income.Payees[0].Amounts["2026-06"] != 1000 {
		t.Errorf("income payees wrong: %+v", report.Income.Payees)
	}
}

func TestAssembleCashFlowReport(t *testing.T) {
	months := []string{"2026-01", "2026-02"}
	salary, startBal := uuid.New(), uuid.New()
	funds, loan := uuid.New(), uuid.New()

	incomeRows := []model.CashFlowIncomeRow{
		{Month: "2026-01", PayeeID: startBal, PayeeName: "Starting Balance", Opening: true, Amount: 5000},
		{Month: "2026-01", PayeeID: salary, PayeeName: "Employer", Amount: 100},
		{Month: "2026-02", PayeeID: salary, PayeeName: "Employer", Amount: 100},
	}
	transferRows := []model.TransferFlowRow{
		{Month: "2026-01", AccountID: loan, AccountName: "Home loan", AccountType: "mortgage", Out: 10},
		{Month: "2026-01", AccountID: funds, AccountName: "Funds", AccountType: "asset", Out: 40},
		{Month: "2026-02", AccountID: funds, AccountName: "Funds", AccountType: "asset", In: 30, RecategorizedCount: 1, RecategorizedAmount: 30},
	}

	report := assembleCashFlowReport(months, incomeRows, nil, transferRows)

	if len(report.Income.Payees) != 1 || report.Income.Payees[0].PayeeID != salary {
		t.Fatalf("income payees = %+v, want only the employer", report.Income.Payees)
	}
	if report.Income.Totals["2026-01"] != 100 {
		t.Errorf("income 2026-01 = %v, want 100 (opening balance excluded)", report.Income.Totals["2026-01"])
	}
	if report.OpeningBalances["2026-01"] != 5000 || report.OpeningBalances["2026-02"] != 0 {
		t.Errorf("openingBalances = %v", report.OpeningBalances)
	}
	if len(report.Transfers) != 2 {
		t.Fatalf("transfers = %d, want 2", len(report.Transfers))
	}
	got := report.Transfers[0]
	if got.AccountID != funds || got.Kind != "asset" {
		t.Fatalf("first transfer = %+v, want Funds (largest) as asset", got)
	}
	wantIn := map[string]float64{"2026-02": 30, "2026-01": 0}
	wantOut := map[string]float64{"2026-01": 40, "2026-02": 0}
	if !reflect.DeepEqual(got.In, wantIn) || !reflect.DeepEqual(got.Out, wantOut) {
		t.Errorf("funds in/out = %v / %v, want %v / %v", got.In, got.Out, wantIn, wantOut)
	}
	if got.RecategorizedCount != 1 || got.RecategorizedAmount != 30 {
		t.Errorf("recategorized = %d / %v, want 1 / 30", got.RecategorizedCount, got.RecategorizedAmount)
	}
	if report.Transfers[1].Kind != "debt" {
		t.Errorf("mortgage kind = %q, want debt", report.Transfers[1].Kind)
	}
}

func TestLoanSchedule(t *testing.T) {
	near := func(got, want float64) bool { return math.Abs(got-want) < 0.01 }

	t.Run("payments clear accrued interest first", func(t *testing.T) {
		loan := model.LoanBalanceInput{
			StartDate:       "2026-01-23",
			OriginalBalance: 120000,
			InterestRate:    12, // 1% a month
			MonthlyPayment:  5000,
			Payments:        map[string]float64{"2026-02": 6000, "2026-03": 0},
		}
		months := loanSchedule(loan, "2026-04")
		if len(months) != 4 {
			t.Fatalf("months = %d, want 4", len(months))
		}
		broken := 120000 * 0.12 * 9 / 365 // 23rd–31st Jan, paid with the Feb payment
		jan, feb, mar, apr := months[0], months[1], months[2], months[3]
		if jan.Payment != 0 || !near(jan.Balance, 120000) {
			t.Errorf("jan = %+v, want no payment and full principal", jan)
		}
		if !near(feb.Interest, broken) || !near(feb.Principal, 6000-broken) || !near(feb.Balance, 120000-(6000-broken)) {
			t.Errorf("feb = %+v, want interest %.2f", feb, broken)
		}
		// March pays nothing (missed payments after the first are not assumed);
		// April's empty payment leaves two months of interest accrued, not
		// added to principal
		if mar.Payment != 0 || apr.Payment != 0 || !near(apr.Balance, feb.Balance) || mar.Assumed {
			t.Errorf("mar = %+v apr = %+v, want no payments and principal unchanged", mar, apr)
		}
		if _, ok := amortizeLoan(loan, "2026-04")["2025-12"]; ok {
			t.Error("no balance expected before the loan started")
		}
	})

	t.Run("assumes scheduled payments before tracking began", func(t *testing.T) {
		loan := model.LoanBalanceInput{
			StartDate:       "2025-11-01",
			OriginalBalance: 10000,
			InterestRate:    0,
			MonthlyPayment:  1000,
			Payments:        map[string]float64{"2026-02": 1000},
		}
		months := loanSchedule(loan, "2026-02")
		// Dec and Jan assumed; Feb recorded
		if !months[1].Assumed || !months[2].Assumed || months[3].Assumed || !near(months[3].Balance, 7000) {
			t.Errorf("months = %+v, want Dec/Jan assumed and 7000 left", months)
		}
	})

	t.Run("paid off never goes negative", func(t *testing.T) {
		loan := model.LoanBalanceInput{
			StartDate:       "2026-01-01",
			OriginalBalance: 1000,
			Payments:        map[string]float64{"2026-02": 5000},
		}
		if got := amortizeLoan(loan, "2026-02"); got["2026-02"] != 0 {
			t.Errorf("feb balance = %v, want 0", got["2026-02"])
		}
	})

	t.Run("bad start date", func(t *testing.T) {
		if got := loanSchedule(model.LoanBalanceInput{StartDate: "soon"}, "2026-02"); len(got) != 0 {
			t.Errorf("months = %v, want none", got)
		}
	})
}
