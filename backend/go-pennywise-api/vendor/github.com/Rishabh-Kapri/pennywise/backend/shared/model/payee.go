package model

import (
	"time"

	"github.com/google/uuid"
)

type Payee struct {
	ID                  uuid.UUID  `json:"id"`
	Name                string     `json:"name"`
	BudgetID            uuid.UUID  `json:"budgetId"`
	TransferAccountID   *uuid.UUID `json:"transferAccountId,omitempty"`
	Deleted             bool       `json:"deleted"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type PayeeRule struct {
	ID          uuid.UUID `json:"id"`
	BudgetID    uuid.UUID `json:"budgetId"`
	PayeeID     uuid.UUID `json:"payeeId"`
	CategoryID  uuid.UUID `json:"categoryId"`
	MatchString string    `json:"matchString"`
	MatchType   string    `json:"matchType"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Deleted     bool      `json:"deleted"`
}
