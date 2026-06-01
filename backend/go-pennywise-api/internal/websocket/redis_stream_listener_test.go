package websocket

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestWebsocketMessageFromRedisValues(t *testing.T) {
	budgetID := uuid.New()
	userID := uuid.New()
	conversationID := uuid.New()

	message, err := websocketMessageFromRedisValues(map[string]any{
		"eventName":      "agent::chat::text_delta",
		"budgetId":       budgetID.String(),
		"userId":         userID.String(),
		"conversationId": conversationID.String(),
		"data":           "hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if message.EventName != "agent::chat::text_delta" {
		t.Fatalf("expected event name to be decoded, got %q", message.EventName)
	}
	if message.BudgetID != budgetID {
		t.Fatalf("expected budget id %s, got %s", budgetID, message.BudgetID)
	}
	if rawMessageString(t, message.Data) != "hello" {
		t.Fatalf("expected data to be decoded, got %#v", message.Data)
	}
}

func TestWebsocketMessageFromRedisValuesSupportsStructFieldNamesAndBinaryUUID(t *testing.T) {
	budgetID := uuid.New()
	userID := uuid.New()
	conversationID := uuid.New()

	message, err := websocketMessageFromRedisValues(map[string]any{
		"EventName":      "agent::chat::text_delta",
		"BudgetId":       string(budgetID[:]),
		"UserId":         string(userID[:]),
		"ConversationId": string(conversationID[:]),
		"Data":           "delta",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if message.BudgetID != budgetID {
		t.Fatalf("expected binary budget id %s, got %s", budgetID, message.BudgetID)
	}
	if rawMessageString(t, message.Data) != "delta" {
		t.Fatalf("expected data to be decoded, got %#v", message.Data)
	}
}

func TestWebsocketMessageFromRedisValuesRequiresEventName(t *testing.T) {
	_, err := websocketMessageFromRedisValues(map[string]any{
		"budgetId": uuid.NewString(),
		"data":     "hello",
	})
	if err == nil {
		t.Fatal("expected missing eventName to fail")
	}
}

func TestWebsocketMessageFromRedisValuesRequiresValidBudgetID(t *testing.T) {
	_, err := websocketMessageFromRedisValues(map[string]any{
		"eventName": "agent::chat::text_delta",
		"budgetId":  "not-a-uuid",
		"data":      "hello",
	})
	if err == nil {
		t.Fatal("expected invalid budgetId to fail")
	}
}

func rawMessageString(t *testing.T, data json.RawMessage) string {
	t.Helper()

	var decoded string
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("expected raw message string, got %s: %v", data, err)
	}
	return decoded
}
