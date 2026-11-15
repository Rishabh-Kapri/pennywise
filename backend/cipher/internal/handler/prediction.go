package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PredictionHandler interface {
	NormalizeEmailText(c *gin.Context)
	ExtractEmailData(c *gin.Context)
	Predict(c *gin.Context)
	GenerateTransactionEmbedding(c *gin.Context)
	HandleCorrection(c *gin.Context)
	SuggestCategoryDescription(c *gin.Context)
	ExtractStatement(c *gin.Context)
}

type predictionHandler struct {
	predictionService service.PredictionService
}

func NewPredictionHandler(ps service.PredictionService) PredictionHandler {
	return &predictionHandler{predictionService: ps}
}

func (h *predictionHandler) NormalizeEmailText(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		EmailText string `json:"emailText"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.predictionService.SummarizeEmailText(ctx, req.EmailText)
	if err != nil {
		logger.Logger(ctx).Error("email text normalization failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "email text normalization failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": res})
}

func (h *predictionHandler) ExtractEmailData(c *gin.Context) {
	ctx := c.Request.Context()

	var req service.ExtractEmailDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.predictionService.ExtractEmailData(ctx, req)
	if err != nil {
		logger.Logger(ctx).Error("email data extraction failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "email data extraction failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *predictionHandler) Predict(c *gin.Context) {
	ctx := c.Request.Context()

	var req service.PredictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.predictionService.Predict(ctx, req)
	if err != nil {
		logger.Logger(ctx).Error("prediction failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prediction failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *predictionHandler) GenerateTransactionEmbedding(c *gin.Context) {
	ctx := c.Request.Context()

	var req service.TransactionEmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.predictionService.GenerateTransactionEmbedding(ctx, req)
	if err != nil {
		logger.Logger(ctx).Error("transaction embedding generation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate transaction embedding"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *predictionHandler) HandleCorrection(c *gin.Context) {
	ctx := c.Request.Context()

	var req service.CorrectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.predictionService.HandleCorrection(ctx, req); err != nil {
		logger.Logger(ctx).Error("correction handling failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process correction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "correction processed"})
}

func (h *predictionHandler) SuggestCategoryDescription(c *gin.Context) {
	ctx := c.Request.Context()

	var req sharedModel.CategoryDescriptionContext
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.CategoryName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "categoryName is required"})
		return
	}

	description, err := h.predictionService.SuggestCategoryDescription(ctx, req)
	if err != nil {
		logger.Logger(ctx).Error("category description suggestion failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to suggest a category description"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"description": description})
}

// ExtractStatement turns one chunk of statement text into transaction rows.
// The text is a full account history: log failures, never the body.
func (h *predictionHandler) ExtractStatement(c *gin.Context) {
	ctx := c.Request.Context()

	var req sharedModel.StatementExtractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	res, err := h.predictionService.ExtractStatement(ctx, req)
	if err != nil {
		logger.Logger(ctx).Error("statement extraction failed",
			"firstPage", req.FirstPage, "lastPage", req.LastPage, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "statement extraction failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func extractBudgetID(c *gin.Context) (uuid.UUID, error) {
	budgetIDStr := c.GetHeader(utils.HeaderBudgetID)
	if budgetIDStr == "" {
		return uuid.Nil, fmt.Errorf("missing %s header", utils.HeaderBudgetID)
	}
	return uuid.Parse(budgetIDStr)
}
