package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

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
	ctx := c.Request.Context()

	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	budgets, err := h.service.GetAll(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, budgets)
}

func (h *budgetHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	var body model.Budget
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Create(ctx, body.Name, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, nil)
}

func (h *budgetHandler) UpdateById(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "error while parsing ID"})
		return
	}
	var body model.Budget
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = h.service.UpdateById(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}
