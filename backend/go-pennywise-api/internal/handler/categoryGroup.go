package handler

import (
	"net/http"
	"strings"

	"pennywise-api/internal/model"
	"pennywise-api/internal/service"

	utils "pennywise-api/pkg"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CategoryGroupHandler interface {
	List(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	DeleteById(c *gin.Context)
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
	month := strings.TrimSpace(c.Query("month"))
	groups, err := h.service.GetAll(ctx, month)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (h *categoryGroupHandler) Create(c *gin.Context) {
	ctx, err := utils.GetBudgetId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var body model.CategoryGroup
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	err = h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, body)
}

func (h *categoryGroupHandler) Update(c *gin.Context) {
	ctx, err := utils.GetBudgetId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is required"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err!= nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var body model.CategoryGroup
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = h.service.Update(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, body)
}

func (h *categoryGroupHandler) DeleteById(c *gin.Context) {
	ctx, err := utils.GetBudgetId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err!= nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = h.service.DeleteById(ctx, parsedId)
	if err!= nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
