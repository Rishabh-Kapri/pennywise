package model

import (
	"time"

	"github.com/google/uuid"
)

type MonthlyBudget struct {
	ID               uuid.UUID `json:"id"`
	Month            string    `json:"month"`
	BudgetID         uuid.UUID `json:"budgetId"`
	CategoryID       uuid.UUID `json:"categoryId"`
	Budgeted         int       `json:"budgeted"`
	CarryoverBalance int       `json:"carryoverBalance"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
