package handler

import (
	"net/http"

	"pennywise-api/internal/service"
	utils "pennywise-api/pkg"

	"github.com/gin-gonic/gin"
)

type CategoryGroupHandler interface {
	List(c *gin.Context)
	// Create(c *gin.Context)
	// Update(c *gin.Context)
	// Delete(c *gin.Context)
}

type categoryGroupHandler struct {
	service service.CategoryGroupService
}

func NewCategoryGroupHandler(service service.CategoryGroupService) CategoryGroupHandler {
	return &categoryGroupHandler{service: service}
}

func (h *categoryGroupHandler) List(c *gin.Context) {
	ctx, err := utils.GetBudgetId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groups, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}
