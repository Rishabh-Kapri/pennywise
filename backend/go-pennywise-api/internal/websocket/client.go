package websocket

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	UserID   uuid.UUID
	BudgetID uuid.UUID
	Conn     *websocket.Conn
	Send     chan sharedModel.Message
	Done     chan any
}

const (
	// Time allowed to write a message.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than PONG_WAIT.
	pingPeriod = 50 * time.Second
)

// Read reads the message from the websocket connection and sends its to the hub.
// Read is run a per-connection goroutine.
func (c *Client) Read(hub ConnectionHub) {
	defer func() {
		hub.UnregisterClient(c)
		c.Conn.Close()
	}()

	/*
	 * t=0s   read deadline set to now + 60s
	 * t=50s  server sends ping
	 * t=50.x browser automatically sends pong
	 * t=50.x server pong handler extends deadline to now + 60s
	 */
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		// When a pong is received, extend the deadline by pongWait time
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		var msg sharedModel.Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			logger.Logger(context.Background()).
				Debug("error while reading message", "budgetId", c.BudgetID, "userId", c.UserID, "error", err)
			return
		}
		msg.BudgetID = c.BudgetID
		msg.UserID = &c.UserID

		// send to the broadcast channel
		hub.Broadcast(msg, c)
	}
}

// Listen to the Send chan and writes the message to the websocket connection.
// A goroutine is started for each connection.
func (c *Client) Write(hub ConnectionHub) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		hub.UnregisterClient(c)
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// channel closed
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.Conn.WriteJSON(msg)
			if err != nil {
				logger.Logger(context.Background()).
					Error("error while sending message", "event", msg.EventName, "budgetId", msg.BudgetID, "data", msg.Data, "err", err)
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Logger(context.Background()).Error("error in ping", "error", err)
				return
			}
		}
	}
}
