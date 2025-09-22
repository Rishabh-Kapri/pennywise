package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID                    uuid.UUID  `json:"id"`
	BudgetID              uuid.UUID  `json:"budgetId"`
	Date                  string     `json:"date"`
	PayeeID               *uuid.UUID `json:"payeeId,omitempty"`
	CategoryID            *uuid.UUID `json:"categoryId,omitempty"`
	AccountID             *uuid.UUID `json:"accountId,omitempty"`
	AccountName           *string    `json:"accountName,omitempty"`
	PayeeName             *string    `json:"payeeName,omitempty"`
	CategoryName          *string    `json:"categoryName,omitempty"`
	Note                  string     `json:"note"`
	Amount                float64    `json:"amount"`
	Inflow                float64    `json:"inflow"`
	Outflow               float64    `json:"outflow"`
	Balance               float64    `json:"balance"`
	Source                string     `json:"source"`
	TransferAccountID     *uuid.UUID `json:"transferAccountId,omitempty"`
	TransferTransactionID *uuid.UUID `json:"transferTransactionId,omitempty"`
	Deleted               bool       `json:"deleted"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type TransactionFilter struct {
	AccountID  *uuid.UUID
	PayeeID    *uuid.UUID
	CategoryID *uuid.UUID
	StartDate  *string
	EndDate    *string
	Note       *string
}

func (t *Transaction) String() string {
	return fmt.Sprintf(`Transaction{
    ID: %v,
    BudgetID: %v,
    Date: %q,
    PayeeID: %s,
    CategoryID: %s,
    AccountID: %s,
    AccountName: %s,
    PayeeName: %s,
    CategoryName: %s,
    Note: %q,
    Amount: %.2f,
    Inflow: %.2f,
    Outflow: %.2f,
    Balance: %.2f,
    Source: %q,
    TransferAccountID: %s,
    TransferTransactionID: %s,
    Deleted: %t,
    CreatedAt: %v,
    UpdatedAt: %v
}`,
		t.ID, t.BudgetID, t.Date,
		ptrToUUIDString(t.PayeeID), ptrToUUIDString(t.CategoryID), ptrToUUIDString(t.AccountID),
		ptrToString(t.AccountName), ptrToString(t.PayeeName), ptrToString(t.CategoryName),
		t.Note, t.Amount, t.Inflow, t.Outflow, t.Balance, t.Source,
		ptrToUUIDString(t.TransferAccountID), ptrToUUIDString(t.TransferTransactionID),
		t.Deleted, t.CreatedAt, t.UpdatedAt)
}
