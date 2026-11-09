package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/userllm"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/gin-gonic/gin"
)

type LLMProviderHandler interface {
	TestModel(c *gin.Context)
}

type llmProviderHandler struct {
	resolver *userllm.Resolver
}

func NewLLMProviderHandler(resolver *userllm.Resolver) LLMProviderHandler {
	return &llmProviderHandler{resolver: resolver}
}

// TestModel makes a tiny call to a model with the budget owner's saved key. A
// failing model is a 200 with ok=false; only a missing key or bad input is an
// error.
func (h *llmProviderHandler) TestModel(c *gin.Context) {
	var body sharedModel.TestUserLLMModelRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.Slug = strings.TrimSpace(body.Slug)
	if !body.Provider.Valid() || !body.Kind.Valid() || body.Slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a valid provider, kind and model are required"})
		return
	}
	res, err := h.resolver.Test(c.Request.Context(), body)
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
