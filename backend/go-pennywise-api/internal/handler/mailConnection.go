package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MailConnectionHandler interface {
	List(c *gin.Context)
	Connect(c *gin.Context)
	Update(c *gin.Context)
	Disconnect(c *gin.Context)
	BackfillCandidates(c *gin.Context)
	Backfill(c *gin.Context)
}

type mailConnectionHandler struct {
	service service.MailConnectionService
}

func NewMailConnectionHandler(service service.MailConnectionService) MailConnectionHandler {
	return &mailConnectionHandler{service: service}
}

func (h *mailConnectionHandler) List(c *gin.Context) {
	connections, err := h.service.List(c.Request.Context())
	if err != nil {
		writeMailConnectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, connections)
}

type connectMailRequest struct {
	Code string `json:"code" binding:"required,max=4096"`
}

func (h *mailConnectionHandler) Connect(c *gin.Context) {
	var req connectMailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}
	connection, err := h.service.Connect(c.Request.Context(), req.Code)
	if err != nil {
		writeMailConnectionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, connection)
}

type updateMailRequest struct {
	Paused *bool `json:"paused" binding:"required"`
}

func (h *mailConnectionHandler) Update(c *gin.Context) {
	connectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection id"})
		return
	}
	var req updateMailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "paused is required"})
		return
	}
	connection, err := h.service.SetPaused(c.Request.Context(), connectionID, *req.Paused)
	if err != nil {
		writeMailConnectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, connection)
}

func (h *mailConnectionHandler) Disconnect(c *gin.Context) {
	connectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection id"})
		return
	}
	if err := h.service.Disconnect(c.Request.Context(), connectionID); err != nil {
		writeMailConnectionError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type backfillRequest struct {
	ExternalIDs []string `json:"externalIds" binding:"required,min=1,max=500"`
}

func (h *mailConnectionHandler) BackfillCandidates(c *gin.Context) {
	connectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection id"})
		return
	}
	candidates, err := h.service.BackfillCandidates(c.Request.Context(), connectionID)
	if err != nil {
		writeMailConnectionError(c, err)
		return
	}
	if candidates == nil {
		candidates = []service.BackfillCandidate{}
	}
	c.JSON(http.StatusOK, candidates)
}

func (h *mailConnectionHandler) Backfill(c *gin.Context) {
	connectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection id"})
		return
	}
	var req backfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "choose at least one email"})
		return
	}
	result, err := h.service.Backfill(c.Request.Context(), connectionID, req.ExternalIDs)
	if err != nil {
		writeMailConnectionError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, result)
}

// writeMailConnectionError answers in words a user can act on; ingest's own
// message names internal URLs.
func writeMailConnectionError(c *gin.Context, err error) {
	switch {
	case errs.StatusOf(err) == http.StatusNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "That mailbox isn't connected to this budget."})
	case errs.StatusOf(err) == http.StatusConflict:
		c.JSON(http.StatusConflict, gin.H{
			"error": "Google didn't give long-term access. Remove Pennywise at " +
				"myaccount.google.com/permissions, then connect again.",
		})
	case errs.StatusOf(err) == http.StatusBadRequest:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google refused this sign-in. Please try again."})
	case errs.StatusOf(err) == http.StatusBadGateway:
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "Gmail was connected, but Google wouldn't start watching it. Please try again.",
		})
	case errs.HasCode(err, errs.CodeMailNotConfigured), errs.StatusOf(err) == http.StatusServiceUnavailable:
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Email sync isn't set up on this server."})
	default:
		logger.Logger(c.Request.Context()).Error("mail connection failed", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Couldn't reach the mail service. Please try again."})
	}
}
