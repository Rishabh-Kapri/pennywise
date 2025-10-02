package handler

import (
	"context"
	"net/http"

	"pennywise-api/internal/model"
	"pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BudgetHandler interface {
	List(c *gin.Context)
	Create(c *gin.Context)
	UpdateById(c *gin.Context)
}

type budgetHandler struct {
	service service.BudgetService
}

func NewBudgetHandler(service service.BudgetService) BudgetHandler {
	return &budgetHandler{service: service}
}

func (h *budgetHandler) List(c *gin.Context) {
	ctx := context.Background()

	budgets, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, budgets)
}

func (h *budgetHandler) Create(c *gin.Context) {
	ctx := context.Background()

	var body model.Budget
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Create(ctx, body.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, nil)
}

func (h *budgetHandler) UpdateById(c *gin.Context) {
	ctx := context.Background()

	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error while parsing ID"})
		return
	}
	var body model.Budget
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	err = h.service.UpdateById(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}
