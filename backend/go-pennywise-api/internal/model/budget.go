package model

import (
	"time"

	"github.com/google/uuid"
)

type BudgetMetadata struct {
	InflowCategoryID   uuid.UUID `json:"inflowCategoryId" validate:"required"`
	StartingBalPayeeID uuid.UUID `json:"startingBalPayeeId" validate:"required"`
	CCGroupID          uuid.UUID `json:"ccGroupId" validate:"required"`
}

type Budget struct {
	ID         uuid.UUID      `json:"id"`
	Name       string         `json:"name"`
	IsSelected bool           `json:"isSelected"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	Metadata   BudgetMetadata `json:"metadata"`
}

