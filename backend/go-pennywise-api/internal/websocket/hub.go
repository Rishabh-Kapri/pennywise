package websocket

import (
	"context"
	"sync"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type ConnectionHub interface {
	Register(budgetId uuid.UUID, userId uuid.UUID, conn *websocket.Conn) *Client
	UnregisterClient(client *Client)
	UnregisterBudget(budgetId uuid.UUID)
	GetSocketSessions(budgetId uuid.UUID) map[*Client]bool
	// broadcast a budget event to all users
	Broadcast(mesage Message)
	HandleBroadcastMessages()
}

type connectionHub struct {
	mu sync.RWMutex
	// connections are scoped by budgetId, multiple users can listen to same budgetId
	connections      map[uuid.UUID]map[*Client]bool // [budgetId][Client]bool
	broadcastChannel chan (Message)
}

func NewConnectionHub() ConnectionHub {
	return &connectionHub{
		connections:      make(map[uuid.UUID]map[*Client]bool),
		broadcastChannel: make(chan Message),
	}
}

// main broadcast message loop
func (r *connectionHub) HandleBroadcastMessages() {
	for {

		message := <-r.broadcastChannel

		r.mu.RLock()
		clients, ok := r.connections[message.BudgetId]
		r.mu.RUnlock()
		if ok {
			for c := range clients {
				// Don't echo message back to the sender
				// if c.UserId != message {
				c.Send <- message
				// }
			}
		}
	}
}

func (r *connectionHub) Register(budgetId uuid.UUID, userId uuid.UUID, conn *websocket.Conn) *Client {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connections[budgetId] == nil {
		r.connections[budgetId] = make(map[*Client]bool)
	}

	client := Client{
		UserId:   userId,
		BudgetId: budgetId,
		Conn:     conn,
		Send:     make(chan Message),
		Done:     make(chan any),
	}
	r.connections[budgetId][&client] = true

	logger.Logger(context.Background()).Info("new connection", "budgetId", budgetId, "userId", userId)
	return &client
}

func (r *connectionHub) UnregisterClient(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	logger.Logger(context.Background()).
		Info("unregister called for", "budgetId", client.BudgetId, "userId", client.UserId)

	clients, ok := r.connections[client.BudgetId]
	if !ok {
		return
	}
	if _, ok := clients[client]; !ok {
		return
	}
	delete(clients, client)
	// goroutine/component that sends data to the channel owns closing the channel
	close(client.Send)
	_ = client.Conn.Close()

	// if no more clients, remove budgetId too
	if len(clients) == 0 {
		delete(r.connections, client.BudgetId)
	}
}

func (r *connectionHub) UnregisterBudget(budgetId uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if clients, ok := r.connections[budgetId]; ok {
		for client, connected := range clients {
			if connected {
				delete(clients, client)
				close(client.Send)
				_ = client.Conn.Close()
			}
		}
		if len(clients) == 0 {
			delete(r.connections, budgetId)
		}
	}
}

// broadcast a budget event to all users
func (r *connectionHub) Broadcast(message Message) {
	r.broadcastChannel <- message
}

func (r *connectionHub) GetSocketSessions(budgetId uuid.UUID) map[*Client]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clients, ok := r.connections[budgetId]
	if ok {
		sessions := make(map[*Client]bool, len(clients))
		for client, connected := range clients {
			sessions[client] = connected
		}
		return sessions
	}
	return nil
}
