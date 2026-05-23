package handler

import (
	stderrors "errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AgentHandler interface {
	GetConversations(c *gin.Context)
	GetConversationMessages(c *gin.Context)
	CreateRun(c *gin.Context)
	GetRun(c *gin.Context)
	CancelRun(c *gin.Context)
	UpdateConversation(c *gin.Context)
	DeleteConversation(c *gin.Context)
	UpdateConversationMessageContent(c *gin.Context)
	UpdateEntityMetadata(c *gin.Context)
}

type agentHandler struct {
	service service.AgentService
}

func NewAgentHandler(service service.AgentService) AgentHandler {
	return &agentHandler{service: service}
}

func (h *agentHandler) GetConversations(c *gin.Context) {
	ctx := c.Request.Context()

	conversations, err := h.service.GetConversations(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conversations)
}

func (h *agentHandler) GetConversationMessages(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := parseQueryID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	messages, err := h.service.GetConversationMessages(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *agentHandler) CreateRun(c *gin.Context) {
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

func (h *agentHandler) GetRun(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := parseQueryID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	run, err := h.service.GetRun(ctx, id)
	if err != nil {
		c.JSON(agentErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, run)
}

func (h *agentHandler) CancelRun(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := parseQueryID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	run, err := h.service.CancelRun(ctx, id)
	if err != nil {
		c.JSON(agentErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, run)
}

func (h *agentHandler) UpdateConversation(c *gin.Context) {
	ctx := c.Request.Context()

	conversationId, err := parseQueryID(c, "conversationId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var data sharedModel.AgentConversation
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.UpdateConversation(ctx, conversationId, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, "updated")
}

func (h *agentHandler) DeleteConversation(c *gin.Context) {
	ctx := c.Request.Context()

	conversationID, err := parseQueryID(c, "conversationId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.DeleteConversation(ctx, conversationID); err != nil {
		c.JSON(agentErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation deleted"})
}

func (h *agentHandler) UpdateConversationMessageContent(c *gin.Context) {
	ctx := c.Request.Context()

	// id is the conversation_message id
	id, err := parseQueryID(c, "messageId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var data []sharedModel.MessagePart
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.UpdateConversationMessageContent(ctx, id, data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}

func (h *agentHandler) UpdateEntityMetadata(c *gin.Context) {
	ctx := c.Request.Context()

	entity := c.Param("entity")
	id, err := parseQueryID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	err = h.service.UpdateEntityMetadata(ctx, entity, id, data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}

func parseQueryID(c *gin.Context, fieldName string) (uuid.UUID, error) {
	id := c.Param(fieldName)
	if id == "" {
		return uuid.Nil, errs.New(errs.CodeInvalidArgument, "invalid query %s", fieldName)
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, errs.New(errs.CodeInvalidArgument, "invalid query %s", fieldName)
	}

	return parsedID, nil
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
