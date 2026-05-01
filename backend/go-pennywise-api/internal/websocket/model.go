package websocket

import (
	"github.com/google/uuid"
)

type Message struct {
	EventName string    `json:"eventName"`
	Data      any       `json:"data"`
	BudgetId  uuid.UUID `json:"budgetId"`
}
