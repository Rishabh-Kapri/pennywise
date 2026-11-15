package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PredictionHandler struct {
	service service.PredictionService
}

func NewPredictionHandler(service service.PredictionService) *PredictionHandler {
	return &PredictionHandler{service: service}
}

func (h *PredictionHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	predictions, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, predictions)
}

func (h *PredictionHandler) ListCipherPredictions(c *gin.Context) {
	ctx := c.Request.Context()

	predictions, err := h.service.GetAllCipherPredictions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if predictions == nil {
		predictions = []model.CipherPredictionRecord{}
	}
	c.JSON(http.StatusOK, predictions)
}

func (h *PredictionHandler) GetByTransactionID(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := c.Params.Get("transactionId")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transactionId is needed"})
		return
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing transactionId"})
		return
	}

	details, err := h.service.GetByTransactionID(ctx, parsedID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, details)
}

func (h *PredictionHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var body model.Prediction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	logger.Logger(ctx).Info("creating prediction")
	createdPredictions, err := h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, createdPredictions)
}

func (h *PredictionHandler) DeleteById(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while parsing id"})
	}
	err = h.service.DeleteById(ctx, parsedId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}

func (h *PredictionHandler) Routes(r gin.IRouter) {
	r.GET("/predictions", h.List)
	r.GET("/predictions/cipher", h.ListCipherPredictions)
	r.GET("/predictions/transactions/:transactionId", h.GetByTransactionID)
	r.POST("/predictions", h.Create)
	r.DELETE("/predictions/:id", h.DeleteById)
}
