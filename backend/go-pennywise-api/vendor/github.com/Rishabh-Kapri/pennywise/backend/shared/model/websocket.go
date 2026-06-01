package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Message struct {
	EventName string          `json:"eventName"`
	Data      json.RawMessage `json:"data"`
	BudgetID  uuid.UUID       `json:"budgetId"`
	UserID    *uuid.UUID      `json:"userId,omitempty"`
	RoomID    *string         `json:"roomId,omitempty"`
}
