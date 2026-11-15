package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AIModelHandler struct {
	service service.AIModelService
}

func NewAIModelHandler(service service.AIModelService) *AIModelHandler {
	return &AIModelHandler{service: service}
}

func aiModelErrorStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrInvalidAIModel), errors.Is(err, service.ErrInvalidAIProvider):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrAIModelTestFailed):
		return http.StatusUnprocessableEntity
	case errors.Is(err, service.ErrAIProviderReadOnly):
		return http.StatusForbidden
	case errors.Is(err, repository.ErrLLMModelNotFound), errors.Is(err, repository.ErrLLMProviderNotFound):
		return http.StatusNotFound
	case errors.Is(err, repository.ErrLLMModelExists):
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// userAndID reads the user and the :id param, writing the error response
// itself when either is missing.
func userAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	userID, err := utils.UserIDFromContext(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return uuid.Nil, uuid.Nil, false
	}
	return userID, id, true
}

func (h *AIModelHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	models, err := h.service.List(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, models)
}

func (h *AIModelHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	var body model.CreateUserLLMModelRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := h.service.Create(ctx, userID, body)
	if err != nil {
		c.JSON(aiModelErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *AIModelHandler) Update(c *gin.Context) {
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	var body model.UpdateUserLLMModelRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := h.service.Update(c.Request.Context(), userID, id, body)
	if err != nil {
		c.JSON(aiModelErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *AIModelHandler) SetDefault(c *gin.Context) {
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	m, err := h.service.SetDefault(c.Request.Context(), userID, id)
	if err != nil {
		c.JSON(aiModelErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *AIModelHandler) SetEnabled(c *gin.Context) {
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	var body model.SetUserLLMModelEnabledRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := h.service.SetEnabled(c.Request.Context(), userID, id, *body.Enabled)
	if err != nil {
		c.JSON(aiModelErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *AIModelHandler) Reorder(c *gin.Context) {
	ctx := c.Request.Context()
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	var body model.ReorderUserLLMModelsRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.Reorder(ctx, userID, body); err != nil {
		c.JSON(aiModelErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AIModelHandler) Delete(c *gin.Context) {
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), userID, id); err != nil {
		c.JSON(aiModelErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AIModelHandler) Test(c *gin.Context) {
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	res, err := h.service.Test(c.Request.Context(), userID, id)
	if err != nil {
		status := aiModelErrorStatus(err)
		if status == http.StatusInternalServerError {
			status = http.StatusBadGateway
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AIModelHandler) Routes(r gin.IRouter) {
	r.GET("/ai-models", h.List)
	r.POST("/ai-models", h.Create)
	r.PUT("/ai-models/order", h.Reorder)
	r.PUT("/ai-models/:id", h.Update)
	r.PUT("/ai-models/:id/enabled", h.SetEnabled)
	r.POST("/ai-models/:id/default", h.SetDefault)
	r.POST("/ai-models/:id/test", h.Test)
	r.DELETE("/ai-models/:id", h.Delete)
}
