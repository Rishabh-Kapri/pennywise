package model

import (
	"time"
)

type Account struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	BudgetID        string    `json:"budgetId"`
	TransferPayeeID string    `json:"transferPayeeId"`
	Type            string    `json:"type"`
	Closed          bool      `json:"closed"`
	Deleted         bool      `json:"deleted"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
