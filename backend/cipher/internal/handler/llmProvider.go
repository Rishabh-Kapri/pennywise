package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/userllm"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/gin-gonic/gin"
)

type LLMProviderHandler interface {
	Test(c *gin.Context)
}

type llmProviderHandler struct {
	resolver *userllm.Resolver
}

func NewLLMProviderHandler(resolver *userllm.Resolver) LLMProviderHandler {
	return &llmProviderHandler{resolver: resolver}
}

type testLLMProviderRequest struct {
	Provider sharedModel.LLMProviderName `json:"provider" binding:"required"`
}

// Test makes a one-off call with the budget owner's saved provider. A failing
// provider is a 200 with ok=false; only a missing row or bad input is an error.
func (h *llmProviderHandler) Test(c *gin.Context) {
	var body testLLMProviderRequest
	if err := c.ShouldBindJSON(&body); err != nil || !body.Provider.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a valid provider is required"})
		return
	}
	res, err := h.resolver.Test(c.Request.Context(), body.Provider)
	if errors.Is(err, repository.ErrLLMProviderNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}
