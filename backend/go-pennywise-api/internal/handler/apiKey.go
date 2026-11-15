package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
)

type APIKeyHandler struct {
	service service.APIKeyService
}

func NewAPIKeyHandler(service service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{service: service}
}

func (h *APIKeyHandler) Create(c *gin.Context) {
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

func (h *APIKeyHandler) GetAll(c *gin.Context) {
	ctx := c.Request.Context()

	keys, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, keys)
}

func (h *APIKeyHandler) GetByKeyID(c *gin.Context) {
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

func (h *APIKeyHandler) Routes(r gin.IRouter) {
	r.GET("/keys", h.GetAll)
	r.GET("/keys/:keyID", h.GetByKeyID)
	r.POST("/keys", middleware.RequireScope(model.ScopeAdmin), h.Create)
}
