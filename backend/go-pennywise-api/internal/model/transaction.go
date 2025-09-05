package model

import (
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
