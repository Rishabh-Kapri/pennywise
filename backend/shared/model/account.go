package model

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	BudgetID        uuid.UUID  `json:"budgetId"`
	TransferPayeeID *uuid.UUID `json:"transferPayeeId,omitempty"`
	Type            string     `json:"type"`
	Bank            string     `json:"bank,omitempty"`   // logo id ("hdfc"); empty when unknown
	Suffix          string     `json:"suffix,omitempty"` // last 4 digits of the account/card number
	Balance         float64    `json:"balance,omitempty"`
	Closed          bool       `json:"closed"`
	Deleted         bool       `json:"deleted"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type AccountSimplified struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// accountTypes are the account types the clients offer. Checking, savings and
// credit cards are on budget; the rest (assets, liabilities, loans) are
// tracked off budget. The report queries in adapters/postgres/report.go list
// the on-budget ones in SQL, so change both together.
var accountTypes = map[string]bool{
	"checking": true, "savings": true, "creditCard": true,
	"asset": true, "liability": true,
	"loan": true, "mortgage": true, "autoLoan": true,
	"studentLoan": true, "personalLoan": true, "medicalDebt": true,
}

// ValidAccountType reports whether t is an account type the clients offer.
func ValidAccountType(t string) bool { return accountTypes[t] }

// OnBudget reports whether the account's money is budgeted by category.
func (a Account) OnBudget() bool {
	return a.Type == "checking" || a.Type == "savings" || a.Type == "creditCard"
}

// TransferPayeeName is the name of the payee that moves money into an account.
func TransferPayeeName(accountName string) string { return "Transfer : " + accountName }
