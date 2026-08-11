package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PredictionReviewHandler interface {
	Queue(c *gin.Context)
	Review(c *gin.Context)
}

type predictionReviewHandler struct {
	service service.PredictionReviewService
}

func NewPredictionReviewHandler(service service.PredictionReviewService) PredictionReviewHandler {
	return &predictionReviewHandler{service: service}
}

func (h *predictionReviewHandler) Queue(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	includeReviewed := c.Query("includeReviewed") == "true"
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	items, err := h.service.GetQueue(ctx, includeReviewed, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *predictionReviewHandler) Review(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var body model.PredictionReviewRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.service.Review(ctx, id, body)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "prediction not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}
