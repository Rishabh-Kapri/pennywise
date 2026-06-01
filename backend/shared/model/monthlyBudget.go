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
	Budgeted         float64   `json:"budgeted"`
	CarryoverBalance float64   `json:"carryoverBalance"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
