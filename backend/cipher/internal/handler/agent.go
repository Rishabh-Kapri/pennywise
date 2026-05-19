package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/gin-gonic/gin"
)

type AgentHandler interface {
	CreateRun(c *gin.Context)
	GetRun(c *gin.Context)
	CancelRun(c *gin.Context)
}

type agentHandler struct {
	service service.AgentService
}

func NewAgentHandler(agentService service.AgentService) AgentHandler {
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
		var clarify *service.ClarifyError
		if errors.As(err, &clarify) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"clarify": clarify.Prompt})
			return
		}
		logger.Logger(ctx).Error("agent run creation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "agent run creation failed"})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *agentHandler) GetRun(c *gin.Context) {
}

func (h *agentHandler) CancelRun(c *gin.Context) {
}
