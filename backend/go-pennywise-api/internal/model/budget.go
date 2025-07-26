package model

import (
	"time"
)

type Budget struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	IsSelected bool      `json:"isSelected"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
