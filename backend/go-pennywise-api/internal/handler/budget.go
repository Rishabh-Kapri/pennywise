package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
)

type BudgetHandler struct {
	service service.BudgetService
}

func NewBudgetHandler(service service.BudgetService) *BudgetHandler {
	return &BudgetHandler{service: service}
}

func (h *BudgetHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := currentUser(c)
	if !ok {
		return
	}

	budgets, err := h.service.GetAll(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if budgets == nil {
		budgets = []model.Budget{} // a new user: clients expect a list, not null
	}
	c.JSON(http.StatusOK, budgets)
}

func (h *BudgetHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := currentUser(c)
	if !ok {
		return
	}

	var body model.CreateBudgetRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	budget, err := h.service.Create(ctx, body, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, budget)
}

func (h *BudgetHandler) UpdateById(c *gin.Context) {
	ctx := c.Request.Context()

	parsedId, ok := pathID(c, "id")
	if !ok {
		return
	}
	var body model.Budget
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := h.service.UpdateById(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}

func (h *BudgetHandler) Routes(r gin.IRouter) {
	r.GET("/budgets", h.List)
	r.POST("/budgets", h.Create)
	r.PATCH("/budgets/:id", h.UpdateById)
}
