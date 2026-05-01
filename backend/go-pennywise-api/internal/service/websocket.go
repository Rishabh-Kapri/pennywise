package service

import (
	"context"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/websocket"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"

	ws "github.com/gorilla/websocket"
)

type WebsocketService interface {
	Connect(ctx context.Context, w http.ResponseWriter, r *http.Request) error
	SendNotification(ctx context.Context, budgetId uuid.UUID, eventName string, data any) error
	GetSessions(ctx context.Context) WebsocketSessionsResponse
	SendTestEvent(ctx context.Context, eventName string, data any) error
}

type WebsocketSession struct {
	UserId   uuid.UUID `json:"userId"`
	BudgetId uuid.UUID `json:"budgetId"`
}

type WebsocketSessionsResponse struct {
	Count    int                `json:"count"`
	Sessions []WebsocketSession `json:"sessions"`
}

type websocketService struct {
	hub websocket.ConnectionHub
}

func NewWebsocketService(hub websocket.ConnectionHub) WebsocketService {
	return &websocketService{hub: hub}
}

// Upgrader is used to upgrade HTTP connections to Websocket connection
var upgrader = ws.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *websocketService) Connect(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	userId := utils.MustUserID(ctx)
	budgetId := utils.MustBudgetID(ctx)

	log := logger.Logger(ctx)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("error while upgrading http -> ws", "error", err)
		return nil
	}
	client := s.hub.Register(budgetId, userId, conn)
	go client.Read(s.hub)
	go client.Write(s.hub)

	// go handleConnection(ctx, conn)

	return nil
}

func (s *websocketService) SendNotification(
	ctx context.Context,
	budgetId uuid.UUID,
	eventName string,
	data any,
) error {
	message := websocket.Message{
		EventName: eventName,
		Data:      data,
		BudgetId:  budgetId,
	}
	s.hub.Broadcast(message)
	return nil
}

func (s *websocketService) GetSessions(ctx context.Context) WebsocketSessionsResponse {
	budgetId := utils.MustBudgetID(ctx)
	clients := s.hub.GetSocketSessions(budgetId)
	sessions := make([]WebsocketSession, 0, len(clients))

	for client, connected := range clients {
		if !connected {
			continue
		}
		sessions = append(sessions, WebsocketSession{
			UserId:   client.UserId,
			BudgetId: client.BudgetId,
		})
	}

	return WebsocketSessionsResponse{
		Count:    len(sessions),
		Sessions: sessions,
	}
}

func (s *websocketService) SendTestEvent(ctx context.Context, eventName string, data any) error {
	budgetId := utils.MustBudgetID(ctx)
	_ = utils.MustUserID(ctx)
	return s.SendNotification(ctx, budgetId, eventName, data)
}
