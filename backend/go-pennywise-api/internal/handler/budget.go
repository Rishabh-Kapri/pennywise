package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
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
	if budgets == nil {
		budgets = []model.Budget{} // a new user: clients expect a list, not null
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

// UpdateById is PATCH /budgets/:id: rename (owner) and select (any member).
func (h *budgetHandler) UpdateById(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
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
	var body model.UpdateBudgetRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.Update(ctx, userID, parsedId, body); err != nil {
		writeStatusError(c, err)
		return
	}
	c.JSON(http.StatusOK, nil)
}

// writeStatusError answers with the status an errs.Error carries (500 when it
// has none) and its code.
func writeStatusError(c *gin.Context, err error) {
	status := errs.StatusOf(err)
	if status == 0 {
		status = http.StatusInternalServerError
	}
	body := gin.H{"error": err.Error()}
	var e *errs.Error
	if errors.As(err, &e) {
		body["error"] = e.Message
		body["code"] = e.Code
	}
	c.JSON(status, body)
}
