package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Websocket event names. Must stay in sync with
// react-frontend/src/features/websocket/events.ts.
const (
	EventTransactionCreated = "pennywise::transaction::created"
)

type Message struct {
	EventName string          `json:"eventName"`
	Data      json.RawMessage `json:"data"`
	BudgetID  uuid.UUID       `json:"budgetId"`
	UserID    *uuid.UUID      `json:"userId,omitempty"`
	RoomID    *string         `json:"roomId,omitempty"`
}
