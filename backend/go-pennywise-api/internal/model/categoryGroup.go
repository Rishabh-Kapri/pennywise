package model

import "time"

type CategoryGroup struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	BudgetID        string         `json:"budgetId"`
	Hidden          bool           `json:"hidden"`
	IsSystem        bool           `json:"isSystem"`
	Deleted         bool           `json:"deleted"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}
