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
	Budgeted        map[string]float64 `json:"budgeted,omitempty"`
	Activity        map[string]float64 `json:"activity,omitempty"`
	Balance         map[string]float64 `json:"balance,omitempty"`
	Note            string             `json:"note"`
	Description     string             `json:"description"`
	Goal            *CategoryGoal      `json:"goal"`
	Hidden          bool               `json:"hidden"`
	IsSystem        bool               `json:"isSystem"`
	Deleted         bool               `json:"deleted"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}

// CategoryDescriptionContext is what a description suggestion is drawn from:
// the category, the group it sits in, its neighbours (so the description can
// say what does not belong) and where its money actually goes.
type CategoryDescriptionContext struct {
	CategoryName       string   `json:"categoryName"`
	GroupName          string   `json:"groupName"`
	SiblingCategories  []string `json:"siblingCategories"`
	TopPayees          []string `json:"topPayees"`
	CurrentDescription string   `json:"currentDescription"`
}

// CategoryDetailsUpdate edits a category's free-text fields. A nil field is
// left unchanged, so the note and description can be saved independently.
type CategoryDetailsUpdate struct {
	Note        *string `json:"note"`
	Description *string `json:"description"`
}

type CategorySimplified struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"` // what belongs in the category; may be empty
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
