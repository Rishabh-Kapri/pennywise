package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	BudgetID  uuid.UUID `json:"budgetId"`
	Email     string    `json:"email"`
	HistoryID uint64    `json:"historyId"`
	Deleted   bool      `json:"deleted"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
