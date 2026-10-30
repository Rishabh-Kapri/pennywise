package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
)

type AIProviderHandler interface {
	List(c *gin.Context)
	Upsert(c *gin.Context)
	Delete(c *gin.Context)
	Test(c *gin.Context)
}

type aiProviderHandler struct {
	service service.AIProviderService
}

func NewAIProviderHandler(service service.AIProviderService) AIProviderHandler {
	return &aiProviderHandler{service: service}
}

func aiProviderErrorStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrInvalidAIProvider), errors.Is(err, service.ErrAIProviderKeyRequired):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrAIProviderReadOnly):
		return http.StatusForbidden
	case errors.Is(err, repository.ErrLLMProviderNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrAIProvidersDisabled):
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

func (h *aiProviderHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	providers, err := h.service.List(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, providers)
}

func (h *aiProviderHandler) Upsert(c *gin.Context) {
	ctx := c.Request.Context()
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	var body model.UpsertUserLLMProviderRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	provider, err := h.service.Upsert(ctx, userID, model.LLMProviderName(c.Param("provider")), body)
	if err != nil {
		c.JSON(aiProviderErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, provider)
}

func (h *aiProviderHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	if err := h.service.Delete(ctx, userID, model.LLMProviderName(c.Param("provider"))); err != nil {
		c.JSON(aiProviderErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *aiProviderHandler) Test(c *gin.Context) {
	res, err := h.service.Test(c.Request.Context(), model.LLMProviderName(c.Param("provider")))
	if err != nil {
		status := aiProviderErrorStatus(err)
		if status == http.StatusInternalServerError {
			status = http.StatusBadGateway
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}
