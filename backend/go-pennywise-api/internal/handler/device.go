package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
)

type DeviceHandler interface {
	RegisterPushToken(c *gin.Context)
	UnregisterPushToken(c *gin.Context)
}

type deviceHandler struct {
	service service.PushNotificationService
}

func NewDeviceHandler(service service.PushNotificationService) DeviceHandler {
	return &deviceHandler{service: service}
}

func (h *deviceHandler) RegisterPushToken(c *gin.Context) {
	ctx := c.Request.Context()

	var body model.DevicePushTokenReq
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.service.RegisterToken(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, token)
}

func (h *deviceHandler) UnregisterPushToken(c *gin.Context) {
	ctx := c.Request.Context()

	var body model.DevicePushTokenReq
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UnregisterToken(ctx, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}
