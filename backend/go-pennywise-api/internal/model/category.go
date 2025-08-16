package model

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID              uuid.UUID          `json:"id"`
	Name            string             `json:"name"`
	BudgetID        uuid.UUID          `json:"budgetId"`
	CategoryGroupID uuid.UUID          `json:"categoryGroupId"`
	Budgeted        map[string]float32 `json:"budgeted,omitempty"`
	Activity        map[string]float32 `json:"activity,omitempty"`
	Balance         map[string]float32 `json:"balance,omitempty"`
	Note            string             `json:"note"`
	Hidden          bool               `json:"hidden"`
	IsSystem        bool               `json:"isSystem"`
	Deleted         bool               `json:"deleted"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}
