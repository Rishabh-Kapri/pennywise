package model

import (
	"time"

	"github.com/google/uuid"
)

type CategoryGroup struct {
	ID         uuid.UUID          `json:"id"`
	Name       string             `json:"name"`
	BudgetID   uuid.UUID          `json:"budgetId"`
	Categories []Category         `json:"categories"`
	Budgeted   map[string]float64 `json:"budgeted,omitempty"`
	Activity   map[string]float64 `json:"activity,omitempty"`
	Balance    map[string]float64 `json:"balance,omitempty"`
	Hidden     bool               `json:"hidden"`
	IsSystem   bool               `json:"isSystem"`
	Deleted    bool               `json:"deleted"`
	CreatedAt  time.Time          `json:"createdAt"`
	UpdatedAt  time.Time          `json:"updatedAt"`
}
