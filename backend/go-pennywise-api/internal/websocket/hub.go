package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type ConnectionHub interface {
	Register(budgetId uuid.UUID, userId uuid.UUID, conn *websocket.Conn) *Client
	UnregisterClient(client *Client)
	UnregisterBudget(budgetId uuid.UUID)
	GetSocketSessions(budgetId uuid.UUID) map[*Client]bool
	// broadcast a budget event to all users
	Broadcast(mesage sharedModel.Message, client *Client)
	// handleBroadcastMessages()
}

type connectionHub struct {
	mu sync.RWMutex
	// connections are scoped by budgetId, multiple users can listen to same budgetId
	connections      map[uuid.UUID]map[*Client]bool // [budgetId][Client]bool
	rooms            map[string]map[*Client]bool
	clientToRoom     map[*Client]string // for easier unregister
	broadcastChannel chan (sharedModel.Message)
}

func NewConnectionHub() ConnectionHub {
	return &connectionHub{
		connections:      make(map[uuid.UUID]map[*Client]bool),
		rooms:            make(map[string]map[*Client]bool),
		clientToRoom:     make(map[*Client]string),
		broadcastChannel: make(chan sharedModel.Message),
	}
}

func (r *connectionHub) agentRoomKey(budgetID string, userID string, streamID string, agentKey string) string {
	return fmt.Sprintf("%s:%s:%s/%s", budgetID, userID, agentKey, streamID)
}

// main broadcast message loop
// func (r *connectionHub) handleBroadcastMessages() {
// 	for {
//
// 		message := <-r.broadcastChannel
//
// 		r.mu.RLock()
// 		clients, ok := r.connections[message.BudgetID]
// 		r.mu.RUnlock()
// 		if ok {
// 			for c := range clients {
// 				// Don't echo message back to the sender
// 				// if c.UserId != message {
// 				c.Send <- message
// 				// }
// 			}
// 		}
// 	}
// }

func (r *connectionHub) Register(budgetId uuid.UUID, userId uuid.UUID, conn *websocket.Conn) *Client {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connections[budgetId] == nil {
		r.connections[budgetId] = make(map[*Client]bool)
	}

	client := Client{
		UserID:   userId,
		BudgetID: budgetId,
		Conn:     conn,
		Send:     make(chan sharedModel.Message),
		Done:     make(chan any),
	}
	r.connections[budgetId][&client] = true
	r.clientToRoom[&client] = ""

	logger.Logger(context.Background()).Info("new connection", "budgetId", budgetId, "userId", userId)
	return &client
}

func (r *connectionHub) removeClientFromRoomLocked(client *Client) {
	roomID, ok := r.clientToRoom[client]
	if !ok || roomID == "" {
		return
	}

	delete(r.clientToRoom, client)
	roomClients, ok := r.rooms[roomID]
	if !ok {
		return
	}

	delete(roomClients, client)
	if len(roomClients) == 0 {
		delete(r.rooms, roomID)
	}
}

func (r *connectionHub) UnregisterClient(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	logger.Logger(context.Background()).
		Info("unregister called for", "budgetId", client.BudgetID, "userId", client.UserID)

	clients, ok := r.connections[client.BudgetID]
	if !ok {
		return
	}
	if _, ok := clients[client]; !ok {
		return
	}

	r.removeClientFromRoomLocked(client)
	delete(clients, client)
	// goroutine/component that sends data to the channel owns closing the channel
	close(client.Send)
	_ = client.Conn.Close()

	// if no more clients, remove budgetId too
	if len(clients) == 0 {
		delete(r.connections, client.BudgetID)
	}
}

func (r *connectionHub) UnregisterBudget(budgetId uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if clients, ok := r.connections[budgetId]; ok {
		for client, connected := range clients {
			if connected {
				r.removeClientFromRoomLocked(client)
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

func (r *connectionHub) broadcastToRoom(roomID string, message sharedModel.Message) {
	r.mu.RLock()
	clients, ok := r.rooms[roomID]
	r.mu.RUnlock()
	if ok {
		for c := range clients {
			c.Send <- message
		}
	}
}

func (r *connectionHub) broadcastToBudget(budgetID string, message sharedModel.Message) {
	r.mu.RLock()
	clients, ok := r.connections[message.BudgetID]
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

// broadcast a message to the users based on the message eventName
func (r *connectionHub) Broadcast(message sharedModel.Message, client *Client) {
	logger.Logger(context.Background()).Info("received chat::stream", "message", message)
	switch message.EventName {
	case "pennywise::agent::chat::subscribe":
		// this is a chat subscription event, client is needed
		if client == nil {
			return
		}
		if message.RoomID == nil {
			return
		}
		ctx := context.Background()
		log := logger.Logger(ctx)
		// get the mutex read lock
		var roomData struct {
			Kind     string `json:"kind"`
			StreamID string `json:"streamId"`
			BudgetID string `json:"budgetId"`
			UserID   string `json:"userId"`
			RoomID   string `json:"roomId"`
		}
		err := json.Unmarshal(message.Data, &roomData)
		if err != nil {
			logger.Logger(ctx).Error("error while unmarshalling room data", "error", err)
			return
		}
		log.Info("subscribing to", "room", roomData)

		if roomData.StreamID == "" || roomData.UserID == "" || roomData.BudgetID == "" {
			logger.Logger(ctx).Error("not enough data for the room stream subscribe", "data", roomData)
			return
		}

		if roomData.BudgetID != client.BudgetID.String() || roomData.UserID != client.UserID.String() {
			logger.Logger(ctx).Error("unauthorized user", "message", message)
			return
		}

		// valid payload
		roomID := r.agentRoomKey(message.BudgetID.String(), message.UserID.String(), roomData.StreamID, "chat")

		r.mu.Lock()
		defer r.mu.Unlock()

		// a room exists for client, remove it first
		currentRoomID := r.clientToRoom[client]
		if currentRoomID != "" && currentRoomID != roomID {
			r.removeClientFromRoomLocked(client)
		}

		roomClients, ok := r.rooms[roomID]
		if !ok || roomClients == nil {
			roomClients = make(map[*Client]bool)
			r.rooms[roomID] = roomClients
		}

		roomClients[client] = true
		r.clientToRoom[client] = roomID
		log.Info("subscribed", "roomID", roomID, "roomClients", roomClients, "clientToRooms", r.clientToRoom)

		return
	case "pennywise::agent::chat::stream":
		if message.RoomID == nil {
			return
		}
		r.broadcastToRoom(*message.RoomID, message)
		return
	default:
		r.broadcastToBudget(message.BudgetID.String(), message)
		return
	}
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
