package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type APIKeyHandler interface {
	Create(c *gin.Context)
	GetByKeyID(c *gin.Context)
}

type apiKeyHandler struct {
	service service.APIKeyService
}

func NewAPIKeyHandler(service service.APIKeyService) APIKeyHandler {
	return &apiKeyHandler{service: service}
}

func (h *apiKeyHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var body model.APIKey
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	createdKey, err := h.service.Create(ctx, &body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, createdKey)
}

func (h *apiKeyHandler) GetByKeyID(c *gin.Context) {
	ctx := c.Request.Context()

	keyID := c.Param("keyID")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keyID is required"})
		return
	}

	key, err := h.service.GetByKeyID(ctx, keyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, key)
}
