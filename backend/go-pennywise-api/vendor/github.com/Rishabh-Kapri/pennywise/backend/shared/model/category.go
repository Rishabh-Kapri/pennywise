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

type CategorySimplified struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// CategoryNameMatch is a category scored against a candidate name by trigram
// similarity. Score is pg_trgm's similarity(), in [0, 1], where 1 is an exact
// match once pg_trgm's own normalization (case folding, punctuation and emoji
// treated as word separators) is applied.
type CategoryNameMatch struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Score float64   `json:"score"`
}

type CategoryFilter struct {
	ID              *uuid.UUID
	Name            *string
	CategoryGroupID *uuid.UUID
	IsSystem        *bool
}
