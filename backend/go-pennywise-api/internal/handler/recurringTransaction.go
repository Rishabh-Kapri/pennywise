package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RecurringTransactionHandler struct {
	service service.RecurringTransactionService
}

func NewRecurringTransactionHandler(service service.RecurringTransactionService) *RecurringTransactionHandler {
	return &RecurringTransactionHandler{service: service}
}

func (h *RecurringTransactionHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	rules, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (h *RecurringTransactionHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	var body model.RecurringTransaction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *RecurringTransactionHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var body model.RecurringTransaction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.service.Update(ctx, id, body)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "recurring transaction not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *RecurringTransactionHandler) DeleteById(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.DeleteById(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "recurring transaction not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "recurring transaction deleted"})
}

// Run materializes everything currently due for this budget, so the user
// doesn't have to wait for the next scheduler tick.
func (h *RecurringTransactionHandler) Run(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	result, err := h.service.RunDue(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *RecurringTransactionHandler) Routes(r gin.IRouter) {
	r.GET("/recurring-transactions", h.List)
	r.POST("/recurring-transactions", h.Create)
	r.POST("/recurring-transactions/run", h.Run)
	r.PATCH("/recurring-transactions/:id", h.Update)
	r.DELETE("/recurring-transactions/:id", h.DeleteById)
}
