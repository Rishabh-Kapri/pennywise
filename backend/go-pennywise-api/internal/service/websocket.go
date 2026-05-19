package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/websocket"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"

	ws "github.com/gorilla/websocket"
)

type WebsocketService interface {
	Connect(ctx context.Context, w http.ResponseWriter, r *http.Request) error
	SendNotification(ctx context.Context, budgetId uuid.UUID, eventName string, data any) error
	GetSessions(ctx context.Context) WebsocketSessionsResponse
	SendTestEvent(ctx context.Context, eventName string, data any, roomID *string) error
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
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "failed to marshal data for notification", err)
	}
	message := sharedModel.Message{
		EventName: eventName,
		Data:      dataJSON,
		BudgetID:  budgetId,
	}
	s.hub.Broadcast(message, nil)
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
			UserId:   client.UserID,
			BudgetId: client.BudgetID,
		})
	}

	return WebsocketSessionsResponse{
		Count:    len(sessions),
		Sessions: sessions,
	}
}

func (s *websocketService) SendTestEvent(ctx context.Context, eventName string, data any, roomID *string) error {
	budgetId := utils.MustBudgetID(ctx)
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "failed to marshal data for notification", err)
	}

	message := sharedModel.Message{
		EventName: eventName,
		Data:      dataJSON,
		BudgetID:  budgetId,
	}

	if roomID != nil {
		userID := utils.MustUserID(ctx)
		if resolvedRoomID := testEventRoomID(budgetId, userID, roomID); resolvedRoomID != nil {
			message.UserID = &userID
			message.RoomID = resolvedRoomID
		}
	}

	s.hub.Broadcast(message, nil)
	return nil
}

func testEventRoomID(budgetID uuid.UUID, userID uuid.UUID, roomID *string) *string {
	if roomID == nil {
		return nil
	}

	trimmedRoomID := strings.TrimSpace(*roomID)
	if trimmedRoomID == "" {
		return nil
	}

	roomSuffix := trimmedRoomID
	if roomParts := strings.SplitN(trimmedRoomID, ":", 3); len(roomParts) == 3 {
		roomSuffix = roomParts[2]
	}

	resolvedRoomID := fmt.Sprintf("%s:%s:%s", budgetID, userID, roomSuffix)
	return &resolvedRoomID
}
