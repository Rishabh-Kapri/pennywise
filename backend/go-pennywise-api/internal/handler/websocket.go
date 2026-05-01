package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/gin-gonic/gin"
)

type WebsocketHandler interface {
	Connect(c *gin.Context)
	GetSessions(c *gin.Context)
	SendTestEvent(c *gin.Context)
}

type websocketTestEventRequest struct {
	EventName string `json:"eventName" binding:"required"`
	Data      any    `json:"data"`
}

type websocketHandler struct {
	service service.WebsocketService
}

func NewWebsocketHandler(service service.WebsocketService) WebsocketHandler {
	return &websocketHandler{service: service}
}

func (h *websocketHandler) Connect(c *gin.Context) {
	ctx := c.Request.Context()

	w := c.Writer
	r := c.Request
	h.service.Connect(ctx, w, r)
}

func (h *websocketHandler) GetSessions(c *gin.Context) {
	ctx := c.Request.Context()
	c.JSON(http.StatusOK, h.service.GetSessions(ctx))
}

func (h *websocketHandler) SendTestEvent(c *gin.Context) {
	ctx := c.Request.Context()
	var req websocketTestEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SendTestEvent(ctx, req.EventName, req.Data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "sent"})
}
