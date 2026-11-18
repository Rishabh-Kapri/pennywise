package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type AIUsageHandler interface {
	Summary(c *gin.Context)
}

type aiUsageHandler struct {
	service service.AIUsageService
}

func NewAIUsageHandler(service service.AIUsageService) AIUsageHandler {
	return &aiUsageHandler{service: service}
}

// Summary is GET /ai-usage?days=30&tz=Asia/Kolkata.
func (h *aiUsageHandler) Summary(c *gin.Context) {
	days, err := strconv.Atoi(c.DefaultQuery("days", "30"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid days"})
		return
	}
	summary, err := h.service.Summary(c.Request.Context(), days, c.Query("tz"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidUsageWindow) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}
