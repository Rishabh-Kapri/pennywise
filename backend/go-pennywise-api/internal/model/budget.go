package model

import (
	"time"

	"github.com/google/uuid"
)

type Budget struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	IsSelected bool      `json:"isSelected"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
