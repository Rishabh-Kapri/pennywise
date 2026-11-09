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
// (usually negative) and NetWorth = Assets + Liabilities. Loan accounts
// enter Liabilities through an amortized balance (see LoanBalanceInput),
// not the sum of their transactions.
type NetWorthReport struct {
	StartMonth string          `json:"startMonth"`
	EndMonth   string          `json:"endMonth"`
	Months     []NetWorthPoint `json:"months"`
}

type NetWorthPoint struct {
	Month       string  `json:"month"`
	Assets      float64 `json:"assets"`
	Investments float64 `json:"investments"` // the tracking "asset" accounts within Assets
	Liabilities float64 `json:"liabilities"`
	Loans       float64 `json:"loans"` // the amortized loan balances within Liabilities
	NetWorth    float64 `json:"netWorth"`
}

// CashFlowReport is the response for GET /api/reports/cashflow. It classifies
// on-budget money by what it is rather than by the category it was filed
// under:
//   - Income: Ready to Assign inflows that are not transfers or opening
//     balances (earned money).
//   - Spending: categorized, non-transfer activity (net of refunds).
//   - Transfers: every transfer between an on-budget account and an
//     off-budget one (asset, liability, loan), whatever category its
//     on-budget leg carries. A fund sale is a withdrawal whether it was sent
//     to Ready to Assign or straight to a category.
//
// Months contains every month in the range, gaps included.
type CashFlowReport struct {
	StartMonth      string             `json:"startMonth"`
	EndMonth        string             `json:"endMonth"`
	Months          []string           `json:"months"`
	Income          IncomeMatrix       `json:"income"`
	Spending        ExpenseMatrix      `json:"spending"`
	Transfers       []AccountFlow      `json:"transfers"`
	OpeningBalances map[string]float64 `json:"openingBalances"`
}

// AccountFlow is the money moved between on-budget accounts and one
// off-budget account, from the budget's side.
type AccountFlow struct {
	AccountID uuid.UUID          `json:"accountId"`
	Name      string             `json:"name"`
	Type      string             `json:"type"`
	Kind      string             `json:"kind"` // "asset" or "debt"
	Closed    bool               `json:"closed"`
	In        map[string]float64 `json:"in"`  // came into on-budget accounts
	Out       map[string]float64 `json:"out"` // left on-budget accounts, positive
	// Inflows from this account that were assigned to a regular category
	// instead of Ready to Assign. They count as In all the same.
	RecategorizedCount  int     `json:"recategorizedCount"`
	RecategorizedAmount float64 `json:"recategorizedAmount"`
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

type CashFlowIncomeRow struct {
	Month     string
	PayeeID   uuid.UUID
	PayeeName string
	Opening   bool
	Amount    float64
}

type TransferFlowRow struct {
	Month               string
	AccountID           uuid.UUID
	AccountName         string
	AccountType         string
	Closed              bool
	In                  float64
	Out                 float64
	RecategorizedCount  int
	RecategorizedAmount float64
}

// LoanBalanceInput is what the report needs to amortize one loan account:
// its terms, and the net amount booked to the account per month (payments
// are positive).
type LoanBalanceInput struct {
	AccountID       uuid.UUID
	StartDate       string // loan_start_date, "YYYY-MM-DD"
	OriginalBalance float64
	InterestRate    float64 // annual, percent
	MonthlyPayment  float64
	Payments        map[string]float64
}

// LoanHistory is one loan's amortized history, the response element of
// GET /api/reports/loans. Months run from the loan's start month to the
// current month.
type LoanHistory struct {
	AccountID     uuid.UUID   `json:"accountId"`
	Balance       float64     `json:"balance"` // outstanding at the end of the last month
	PrincipalPaid float64     `json:"principalPaid"`
	InterestPaid  float64     `json:"interestPaid"`
	Months        []LoanMonth `json:"months"`
}

type LoanMonth struct {
	Month     string  `json:"month"`
	Payment   float64 `json:"payment"`
	Interest  float64 `json:"interest"`
	Principal float64 `json:"principal"` // payment - interest
	Balance   float64 `json:"balance"`
	// Assumed is true when no payment was recorded yet and the scheduled
	// payment stood in (months before tracking began).
	Assumed bool `json:"assumed"`
}
