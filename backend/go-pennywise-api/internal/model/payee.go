package model

import (
	"time"
)

type Payee struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	BudgetID          string    `json:"budgetId"`
	TransferAccountID string    `json:"transferAccountId"`
	Deleted           bool      `json:"deleted"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
