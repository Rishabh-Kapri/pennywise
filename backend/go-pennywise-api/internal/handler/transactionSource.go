package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type TransactionSourceHandler interface {
	// ListByTransaction is GET /transactions/:id/sources.
	ListByTransaction(c *gin.Context)
}

type transactionSourceHandler struct {
	service service.TransactionSourceService
}

func NewTransactionSourceHandler(service service.TransactionSourceService) TransactionSourceHandler {
	return &transactionSourceHandler{service: service}
}

func (h *transactionSourceHandler) ListByTransaction(c *gin.Context) {
	id, ok := bindTransactionID(c)
	if !ok {
		return
	}
	sources, err := h.service.ListByTransaction(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sources)
}
