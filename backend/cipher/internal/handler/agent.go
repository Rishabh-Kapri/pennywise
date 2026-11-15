package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/agentrun"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/gin-gonic/gin"
)

type AgentHandler interface {
	CreateRun(c *gin.Context)
}

type agentHandler struct {
	service agentrun.AgentService
}

func NewAgentHandler(agentService agentrun.AgentService) AgentHandler {
	return &agentHandler{service: agentService}
}

func (h *agentHandler) CreateRun(c *gin.Context) {
	ctx := c.Request.Context()

	var req sharedModel.AgentRunCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.service.CreateRun(ctx, req)
	if err != nil {
		var clarify *agentrun.ClarifyError
		if errors.As(err, &clarify) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"clarify": clarify.Prompt})
			return
		}
		logger.Logger(ctx).Error("agent run creation failed", "error", err)
		// internal endpoint: return the real error so pennywise-api can store
		// it on the run and surface it to the UI
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
