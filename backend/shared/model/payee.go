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
	CanonicalMerchantID *uuid.UUID `json:"canonicalMerchantId,omitempty"`
	DefaultCategoryID   *uuid.UUID `json:"defaultCategoryId,omitempty"`
	Deleted             bool       `json:"deleted"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type PayeeMatch struct {
	ID          uuid.UUID `json:"id"`
	BudgetID    uuid.UUID `json:"budget_id"`
	PayeeID     uuid.UUID `json:"payee_id"`
	MatchString string    `json:"match_string"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Deleted     bool      `json:"deleted"`
}
