package websocket

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	UserId   uuid.UUID
	BudgetId uuid.UUID
	Conn     *websocket.Conn
	Send     chan Message
	Done     chan any
}

// Reads the message from the websocket connection hub and sends it
func (c *Client) Read(hub ConnectionHub) {
	defer func() {
		hub.UnregisterClient(c)
		c.Conn.Close()
	}()

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			logger.Logger(context.Background()).
				Error("error while reading message", "budgetId", c.BudgetId, "userId", c.UserId, "error", err)
			break
		}
		msg.BudgetId = c.BudgetId

		// send to the broadcast channel
		hub.Broadcast(msg)
	}
}

// Listen to the Send chan and writes the message to the websocket connection
func (c *Client) Write(hub ConnectionHub) {
	defer hub.UnregisterClient(c)

	for {
		msg, ok := <-c.Send
		if !ok {
			// channel closed
			break
		}

		err := c.Conn.WriteJSON(msg)
		if err != nil {
			logger.Logger(context.Background()).
				Error("error while sending message", "event", msg.EventName, "budgetId", msg.BudgetId, "data", msg.Data, "err", err)
			break
		}
	}
}
