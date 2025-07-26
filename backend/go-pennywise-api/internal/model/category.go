package model

import "time"

type Category struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	BudgetID        string             `json:"budgetId"`
	CategoryGroupID string             `json:"categoryGroupId"`
	Budgeted        map[string]float32 `json:"budgeted"`
	Note            string             `json:"note"`
	Hidden          bool               `json:"hidden"`
	IsSystem        bool               `json:"isSystem"`
	Deleted         bool               `json:"deleted"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}
