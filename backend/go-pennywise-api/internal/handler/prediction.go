package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	utils "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/pkg"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PredictionHandler interface {
	List(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	DeleteById(c *gin.Context)
}

type predictionHandler struct {
	service service.PredictionService
}

func NewPredictionHandler(service service.PredictionService) PredictionHandler {
	return &predictionHandler{service: service}
}

func (h *predictionHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	categories, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *predictionHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var body model.Prediction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.Logger(ctx).Info("creating prediction")
	createdPredictions, err := h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, createdPredictions)
}

func (h *predictionHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while parsing id"})
		return
	}

	var body model.Prediction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	err = h.service.Update(ctx, parsedId, body) // update to parsedId if needed
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, body)
}

func (h *predictionHandler) DeleteById(c *gin.Context) {
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
