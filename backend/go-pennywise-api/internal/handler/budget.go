package handler

import (
	"context"
	"net/http"

	"pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type BudgetHandler interface {
	List(c *gin.Context)
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
