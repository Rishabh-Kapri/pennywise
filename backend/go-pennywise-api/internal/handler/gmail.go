package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type GmailHandler struct{ service service.GmailService }

func NewGmailHandler(service service.GmailService) *GmailHandler {
	return &GmailHandler{service: service}
}

func gmailError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	message := "Gmail controls are unavailable. Please try again."
	var typed *errs.Error
	if errors.Is(err, pgx.ErrNoRows) {
		status, message = http.StatusNotFound, "Gmail connection not found"
	} else if errors.As(err, &typed) {
		message = typed.Message
		switch typed.Code {
		case errs.CodeInvalidArgument:
			status = http.StatusBadRequest
		case errs.CodeHTTPClientError:
			status = http.StatusBadGateway
		}
	}
	c.JSON(status, gin.H{"error": message})
}

func (h *GmailHandler) ListConnections(c *gin.Context) {
	connections, err := h.service.ListConnections(c.Request.Context())
	if err != nil {
		gmailError(c, err)
		return
	}
	c.JSON(http.StatusOK, connections)
}

func (h *GmailHandler) Control(c *gin.Context) {
	var req model.GmailControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a Gmail providerId is required"})
		return
	}
	var err error
	switch c.Param("action") {
	case "pause":
		err = h.service.Pause(c.Request.Context(), req)
	case "resume":
		err = h.service.Resume(c.Request.Context(), req)
	case "sync":
		var result *model.GmailSyncResult
		result, err = h.service.SyncNow(c.Request.Context(), req)
		if err == nil {
			c.JSON(http.StatusAccepted, result)
			return
		}
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown Gmail action"})
		return
	}
	if err != nil {
		gmailError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Gmail connection updated"})
}

func (h *GmailHandler) Reconnect(c *gin.Context) {
	var req model.GmailReconnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "providerId and Google authorization code are required"})
		return
	}
	if err := h.service.Reconnect(c.Request.Context(), req); err != nil {
		gmailError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Gmail reconnected"})
}
