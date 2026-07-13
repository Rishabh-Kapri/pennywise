package service

import (
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
