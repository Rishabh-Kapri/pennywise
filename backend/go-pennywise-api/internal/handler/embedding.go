package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"pennywise-api/internal/model"
	"pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type EmbeddingHandler interface {
	Search(c *gin.Context)
	Create(c *gin.Context)
}

type embeddingHandler struct {
	service service.EmbeddingService
}

func NewEmbeddingHandler(service service.EmbeddingService) EmbeddingHandler {
	return &embeddingHandler{service}
}

func (h *embeddingHandler) Search(c *gin.Context) {
	ctx := context.Background()

	queryStr := strings.TrimSpace(c.Query("query"))
	docTypeQuery := strings.TrimSpace(c.Query("type"))
	limitQuery := c.Query("limit")

	limit := int64(5)
	if limitQuery != "" {
		var err error
		limit, err = strconv.ParseInt(limitQuery, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	docType := "journal_bullet"
	if docTypeQuery != "" {
		docType = docTypeQuery
	}

	docs, err := h.service.Get(ctx, docType, queryStr, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, docs)
}

func (h *embeddingHandler) Create(c *gin.Context) {
	ctx := context.Background()
	var body model.Embedding

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Embedding Created!"})
}
