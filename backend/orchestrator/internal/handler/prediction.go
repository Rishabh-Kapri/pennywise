package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"orchestrator/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PredictionHandler interface {
	Predict(c *gin.Context)
	HandleCorrection(c *gin.Context)
}

type predictionHandler struct {
	predictionService service.PredictionService
}

func NewPredictionHandler(ps service.PredictionService) PredictionHandler {
	return &predictionHandler{predictionService: ps}
}

func (h *predictionHandler) Predict(c *gin.Context) {
	budgetID, err := extractBudgetID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req service.PredictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.predictionService.Predict(c.Request.Context(), budgetID, req)
	if err != nil {
		slog.Error("prediction failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prediction failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *predictionHandler) HandleCorrection(c *gin.Context) {
	var req service.CorrectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.BudgetID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "budgetId is required"})
		return
	}

	if err := h.predictionService.HandleCorrection(c.Request.Context(), req); err != nil {
		slog.Error("correction handling failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process correction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "correction processed"})
}

func extractBudgetID(c *gin.Context) (uuid.UUID, error) {
	budgetIDStr := c.GetHeader("X-Budget-ID")
	if budgetIDStr == "" {
		return uuid.Nil, fmt.Errorf("missing X-Budget-ID header")
	}
	return uuid.Parse(budgetIDStr)
}
