package model

import (
	"fmt"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"

	"github.com/google/uuid"
)

type Date string

// check if date is valid and is in the format YYYY-MM-DD
func (d Date) Valid() error {
	if d == "" {
		return errs.New(errs.CodeInvalidArgument, "date is required")
	}
	_, err := time.Parse("2006-01-02", string(d))
	if err != nil {
		return errs.Wrap(errs.CodeInvalidArgument, "invalid date format", err)
	}
	return nil
}

func (d Date) String() string {
	return string(d)
}

type Transaction struct {
	ID                    uuid.UUID   `json:"id"`
	BudgetID              uuid.UUID   `json:"budgetId"`
	Date                  Date        `json:"date"`
	PayeeID               *uuid.UUID  `json:"payeeId,omitempty"`
	CategoryID            *uuid.UUID  `json:"categoryId,omitempty"`
	AccountID             *uuid.UUID  `json:"accountId,omitempty"`
	AccountName           *string     `json:"accountName,omitempty"`
	PayeeName             *string     `json:"payeeName,omitempty"`
	CategoryName          *string     `json:"categoryName,omitempty"`
	Note                  string      `json:"note"`
	Amount                float64     `json:"amount"`
	Inflow                float64     `json:"inflow"`
	Outflow               float64     `json:"outflow"`
	Balance               float64     `json:"balance"`
	Source                string      `json:"source"`
	TransferAccountID     *uuid.UUID  `json:"transferAccountId,omitempty"`
	TransferTransactionID *uuid.UUID  `json:"transferTransactionId,omitempty"`
	TagIDs                []uuid.UUID `json:"tagIds"`
	Deleted               bool        `json:"deleted"`
	CreatedAt             time.Time   `json:"createdAt"`
	UpdatedAt             time.Time   `json:"updatedAt"`
}

type TransactionFilter struct {
	AccountID  *uuid.UUID
	PayeeID    *uuid.UUID
	CategoryID *uuid.UUID
	StartDate  *string
	EndDate    *string
	Note       *string
}

// Helper method to help log the transaction object
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

// Compare returns true if the transactions are the same for update purposes
func (t *Transaction) Compare(other *Transaction) bool {
	if t.ID.String() != other.ID.String() {
		return false
	}
	if t.Date != other.Date {
		return false
	}
	if ptrToUUIDString(t.PayeeID) != ptrToUUIDString(other.PayeeID) {
		return false
	}
	if ptrToUUIDString(t.CategoryID) != ptrToUUIDString(other.CategoryID) {
		return false
	}
	if ptrToUUIDString(t.AccountID) != ptrToUUIDString(other.AccountID) {
		return false
	}
	if t.Note != other.Note {
		return false
	}
	if t.Amount != other.Amount {
		return false
	}
	// handle tagIds
	if len(t.TagIDs) != len(other.TagIDs) {
		return false
	}

	// @TODO: handle tagIds
	for i, tagId := range t.TagIDs {
		if tagId != other.TagIDs[i] {
			return false
		}
	}

	return true
}
