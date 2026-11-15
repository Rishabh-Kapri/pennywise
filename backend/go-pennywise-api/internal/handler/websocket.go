package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/gin-gonic/gin"
)

type websocketTestEventRequest struct {
	EventName string  `json:"eventName" binding:"required"`
	Data      any     `json:"data"`
	RoomID    *string `json:"roomId,omitempty"`
}

type WebsocketHandler struct {
	service service.WebsocketService
}

func NewWebsocketHandler(service service.WebsocketService) *WebsocketHandler {
	return &WebsocketHandler{service: service}
}

func (h *WebsocketHandler) Connect(c *gin.Context) {
	ctx := c.Request.Context()

	w := c.Writer
	r := c.Request
	h.service.Connect(ctx, w, r)
}

func (h *WebsocketHandler) GetSessions(c *gin.Context) {
	ctx := c.Request.Context()
	c.JSON(http.StatusOK, h.service.GetSessions(ctx))
}

func (h *WebsocketHandler) SendTestEvent(c *gin.Context) {
	ctx := c.Request.Context()
	var req websocketTestEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SendTestEvent(ctx, req.EventName, req.Data, req.RoomID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "sent"})
}

func (h *WebsocketHandler) Routes(r gin.IRouter) {
	r.GET("", h.Connect)
	r.GET("/sessions", h.GetSessions)
	r.POST("/test-event", h.SendTestEvent)
}
