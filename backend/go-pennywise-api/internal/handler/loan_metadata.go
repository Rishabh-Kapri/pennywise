package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type LoanMetadataHandler interface {
	List(c *gin.Context)
	GetByAccountId(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
}

type loanMetadataHandler struct {
	service service.LoanMetadataService
}

func NewLoanMetadataHandler(service service.LoanMetadataService) LoanMetadataHandler {
	return &loanMetadataHandler{service: service}
}

func (h *loanMetadataHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	loans, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, loans)
}

func (h *loanMetadataHandler) GetByAccountId(c *gin.Context) {
	accountId, err := uuid.Parse(c.Param("accountId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account ID"})
		return
	}

	loan, err := h.service.GetByAccountId(c.Request.Context(), accountId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "loan metadata not found"})
		return
	}
	c.JSON(http.StatusOK, loan)
}

func (h *loanMetadataHandler) Create(c *gin.Context) {
	var body model.LoanMetadata
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.service.Create(c.Request.Context(), body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *loanMetadataHandler) Update(c *gin.Context) {
	accountId, err := uuid.Parse(c.Param("accountId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account ID"})
		return
	}

	var body model.LoanMetadata
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.service.Update(c.Request.Context(), accountId, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *loanMetadataHandler) Delete(c *gin.Context) {
	accountId, err := uuid.Parse(c.Param("accountId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account ID"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), accountId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "loan metadata deleted"})
}
