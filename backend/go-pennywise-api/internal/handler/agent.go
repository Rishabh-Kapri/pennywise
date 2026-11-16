package handler

import (
	stderrors "errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	service service.AgentService
}

func NewAgentHandler(service service.AgentService) *AgentHandler {
	return &AgentHandler{service: service}
}

func (h *AgentHandler) GetConversations(c *gin.Context) {
	ctx := c.Request.Context()

	conversations, err := h.service.GetConversations(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conversations)
}

func (h *AgentHandler) GetConversationMessages(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := pathID(c, "id")
	if !ok {
		return
	}

	messages, err := h.service.GetConversationMessages(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *AgentHandler) CreateRun(c *gin.Context) {
	ctx := c.Request.Context()

	var body sharedModel.AgentRunCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	run, err := h.service.CreateRun(ctx, body)
	if err != nil {
		c.JSON(agentErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, run)
}

func (h *AgentHandler) GetRun(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := pathID(c, "id")
	if !ok {
		return
	}

	run, err := h.service.GetRun(ctx, id)
	if err != nil {
		c.JSON(agentErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, run)
}

func (h *AgentHandler) CancelRun(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := pathID(c, "id")
	if !ok {
		return
	}

	run, err := h.service.CancelRun(ctx, id)
	if err != nil {
		c.JSON(agentErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, run)
}

func (h *AgentHandler) UpdateConversation(c *gin.Context) {
	ctx := c.Request.Context()

	conversationId, ok := pathID(c, "conversationId")
	if !ok {
		return
	}

	var data sharedModel.AgentConversation
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.UpdateConversation(ctx, conversationId, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, "updated")
}

func (h *AgentHandler) DeleteConversation(c *gin.Context) {
	ctx := c.Request.Context()

	conversationID, ok := pathID(c, "conversationId")
	if !ok {
		return
	}

	if err := h.service.DeleteConversation(ctx, conversationID); err != nil {
		c.JSON(agentErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation deleted"})
}

func (h *AgentHandler) UpdateConversationMessageContent(c *gin.Context) {
	ctx := c.Request.Context()

	// id is the conversation_message id
	id, ok := pathID(c, "messageId")
	if !ok {
		return
	}

	var data []sharedModel.MessagePart
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.UpdateConversationMessageContent(ctx, id, data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}

func (h *AgentHandler) UpdateEntityMetadata(c *gin.Context) {
	ctx := c.Request.Context()

	entity := c.Param("entity")
	id, ok := pathID(c, "id")
	if !ok {
		return
	}

	var data map[string]any
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if data == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data is required"})
		return
	}
	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data is empty"})
		return
	}

	err := h.service.UpdateEntityMetadata(ctx, entity, id, data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}

func agentErrorStatus(err error) int {
	var apiErr *errs.Error
	if stderrors.As(err, &apiErr) {
		switch apiErr.Code {
		case errs.CodeAgentRunNotFound, errs.CodeAgentConversationNotFound:
			return http.StatusNotFound
		case errs.CodeInvalidArgument:
			return http.StatusBadRequest
		case errs.CodeAgentDispatchFailed, errs.CodeAgentCancelFailed:
			return http.StatusBadGateway
		default:
			return http.StatusInternalServerError
		}
	}

	return http.StatusBadGateway
}

func (h *AgentHandler) Routes(r gin.IRouter) {
	r.GET("/agent/conversations", h.GetConversations)
	r.GET("/agent/conversations/:id/messages", h.GetConversationMessages)
	r.PATCH("/agent/conversations/:conversationId", h.UpdateConversation)
	r.DELETE("/agent/conversations/:conversationId", h.DeleteConversation)
	r.PATCH("/agent/conversations/:conversationId/message/:messageId", h.UpdateConversationMessageContent)
	r.POST("/agent/runs", h.CreateRun)
	r.GET("/agent/runs/:id", h.GetRun)
	r.POST("/agent/runs/:id/cancel", h.CancelRun)
	r.PATCH("/agent/:entity/:id/metadata", h.UpdateEntityMetadata)
}
