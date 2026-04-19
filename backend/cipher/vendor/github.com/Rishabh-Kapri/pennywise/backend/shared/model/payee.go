package model

import (
	"time"

	"github.com/google/uuid"
)

type Payee struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name"`
	BudgetID          uuid.UUID  `json:"budgetId"`
	TransferAccountID *uuid.UUID `json:"transferAccountId,omitempty"`
	Deleted           bool       `json:"deleted"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}
