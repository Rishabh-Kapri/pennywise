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
