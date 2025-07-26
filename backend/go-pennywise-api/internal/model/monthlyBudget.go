package model

import "time"

type MonthlyBudget struct {
	ID               string    `json:"id"`
	Month            string    `json:"month"`
	BudgetID         string    `json:"budgetId"`
	CategoryID       string    `json:"categoryId"`
	Budgeted         int       `json:"budgeted"`
	CarryoverBalance int       `json:"carryoverBalance"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
