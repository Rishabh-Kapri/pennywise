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
	Balance         float64    `json:"balance,omitempty"`
	Closed          bool       `json:"closed"`
	Deleted         bool       `json:"deleted"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}
