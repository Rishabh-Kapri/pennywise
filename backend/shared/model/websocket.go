package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Websocket event names. Must stay in sync with
// react-frontend/src/features/websocket/events.ts.
const (
	EventTransactionCreated = "pennywise::transaction::created"
	// EventPipelineUpdate broadcasts a pipeline_runs row to the budget's
	// websocket clients whenever the run changes.
	EventPipelineUpdate = "pennywise::pipeline::update"
	// EventJobUpdate tells the budget's websocket clients a job changed.
	EventJobUpdate = "pennywise::jobs::update"
)

type Message struct {
	EventName string          `json:"eventName"`
	Data      json.RawMessage `json:"data"`
	BudgetID  uuid.UUID       `json:"budgetId"`
	UserID    *uuid.UUID      `json:"userId,omitempty"`
	RoomID    *string         `json:"roomId,omitempty"`
}
