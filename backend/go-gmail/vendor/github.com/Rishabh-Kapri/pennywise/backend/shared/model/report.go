package model

import (
	"github.com/google/uuid"
)

// ReportParams holds the common query parameters for report endpoints.
type ReportParams struct {
	StartMonth  string // "YYYY-MM", inclusive
	EndMonth    string // "YYYY-MM", inclusive
	AccountIDs  []uuid.UUID
	CategoryIDs []uuid.UUID
	TagIDs      []uuid.UUID
}

// SpendingReport is the response for GET /api/reports/spending.
type SpendingReport struct {
	StartMonth string                `json:"startMonth"`
	EndMonth   string                `json:"endMonth"`
	Total      float64               `json:"total"`
	Groups     []SpendingGroupReport `json:"groups"`
}

type SpendingGroupReport struct {
	CategoryGroupID uuid.UUID                `json:"categoryGroupId"`
	Name            string                   `json:"name"`
	Total           float64                  `json:"total"`
	Categories      []SpendingCategoryReport `json:"categories"`
}

type SpendingCategoryReport struct {
	CategoryID uuid.UUID `json:"categoryId"`
	Name       string    `json:"name"`
	Total      float64   `json:"total"`
}

// IncomeExpenseReport is the response for GET /api/reports/income-expense.
// Months contains every month in the requested range, gaps included.
type IncomeExpenseReport struct {
	StartMonth string             `json:"startMonth"`
	EndMonth   string             `json:"endMonth"`
	Months     []string           `json:"months"`
	Income     IncomeMatrix       `json:"income"`
	Expense    ExpenseMatrix      `json:"expense"`
	Net        map[string]float64 `json:"net"`
}

type IncomeMatrix struct {
	Payees []PayeeMonthlyAmounts `json:"payees"`
	Totals map[string]float64    `json:"totals"`
}

type PayeeMonthlyAmounts struct {
	PayeeID uuid.UUID          `json:"payeeId"`
	Name    string             `json:"name"`
	Amounts map[string]float64 `json:"amounts"`
}

type ExpenseMatrix struct {
	Groups []ExpenseGroupMonthly `json:"groups"`
	Totals map[string]float64    `json:"totals"`
}

type ExpenseGroupMonthly struct {
	CategoryGroupID uuid.UUID                `json:"categoryGroupId"`
	Name            string                   `json:"name"`
	Totals          map[string]float64       `json:"totals"`
	Categories      []CategoryMonthlyAmounts `json:"categories"`
}

type CategoryMonthlyAmounts struct {
	CategoryID uuid.UUID          `json:"categoryId"`
	Name       string             `json:"name"`
	Amounts    map[string]float64 `json:"amounts"`
}

// NetWorthReport is the response for GET /api/reports/networth.
// Values are cumulative as of each month end; liabilities are signed
// (usually negative) and NetWorth = Assets + Liabilities.
type NetWorthReport struct {
	StartMonth string          `json:"startMonth"`
	EndMonth   string          `json:"endMonth"`
	Months     []NetWorthPoint `json:"months"`
}

type NetWorthPoint struct {
	Month       string  `json:"month"`
	Assets      float64 `json:"assets"`
	Liabilities float64 `json:"liabilities"`
	NetWorth    float64 `json:"netWorth"`
}

// Flat rows returned by the report repository; the service assembles
// them into the response matrices above.
type SpendingRow struct {
	GroupID      uuid.UUID
	GroupName    string
	CategoryID   uuid.UUID
	CategoryName string
	Net          float64
}

type IncomeRow struct {
	Month     string
	PayeeID   uuid.UUID
	PayeeName string
	Amount    float64
}

type ExpenseRow struct {
	Month        string
	GroupID      uuid.UUID
	GroupName    string
	CategoryID   uuid.UUID
	CategoryName string
	Net          float64
}
