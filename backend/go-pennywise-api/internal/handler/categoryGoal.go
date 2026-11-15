package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CategoryGoalHandler struct {
	service service.CategoryGoalService
}

func NewCategoryGoalHandler(service service.CategoryGoalService) *CategoryGoalHandler {
	return &CategoryGoalHandler{service: service}
}

// Upsert sets (or replaces) a category's target.
func (h *CategoryGoalHandler) Upsert(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
		return
	}
	var body model.CategoryGoal
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	goal, err := h.service.Upsert(ctx, id, body)
	if err != nil {
		goalError(c, err)
		return
	}
	c.JSON(http.StatusOK, goal)
}

// Delete removes a category's target.
func (h *CategoryGoalHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
		return
	}
	if err := h.service.Delete(ctx, id); err != nil {
		goalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"goal": nil})
}

func goalError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrInvalidGoal):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrCategoryNotFound), errors.Is(err, service.ErrCategoryGoalNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *CategoryGoalHandler) Routes(r gin.IRouter) {
	r.PUT("/categories/:id/goal", h.Upsert)
	r.DELETE("/categories/:id/goal", h.Delete)
}
