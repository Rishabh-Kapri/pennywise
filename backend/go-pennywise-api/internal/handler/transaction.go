package handler

import (
	"net/http"

	"pennywise-api/internal/service"
	utils "pennywise-api/pkg"

	"github.com/gin-gonic/gin"
)

type TransactionHandler interface {
	List(c *gin.Context)
	// Create(c *gin.Context)
	// Update(c *gin.Context)
	// GetById(c *gin.Context)
	// DeleteById(c *gin.Context)
}

type transactionHandler struct {
	service service.TransactionService
}

func NewTransactionHandler(service service.TransactionService) TransactionHandler {
	return &transactionHandler{service: service}
}

func (h *transactionHandler) List(c *gin.Context) {
	ctx, err := utils.GetBudgetId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transactions, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}
