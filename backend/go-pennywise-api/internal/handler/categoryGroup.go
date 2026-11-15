package handler

import (
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
)

type CategoryGroupHandler struct {
	service service.CategoryGroupService
}

func NewCategoryGroupHandler(service service.CategoryGroupService) *CategoryGroupHandler {
	return &CategoryGroupHandler{service: service}
}

func (h *CategoryGroupHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	month := strings.TrimSpace(c.Query("month"))
	groups, err := h.service.GetAll(ctx, month)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (h *CategoryGroupHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var body model.CategoryGroup
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	createdGroup, err := h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, createdGroup)
}

func (h *CategoryGroupHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	parsedId, ok := pathID(c, "id")
	if !ok {
		return
	}
	var body model.CategoryGroup
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := h.service.Update(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, body)
}

func (h *CategoryGroupHandler) DeleteById(c *gin.Context) {
	ctx := c.Request.Context()
	parsedId, ok := pathID(c, "id")
	if !ok {
		return
	}
	err := h.service.DeleteById(ctx, parsedId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *CategoryGroupHandler) Routes(r gin.IRouter) {
	r.GET("/category-groups", h.List)
	r.POST("/category-groups", h.Create)
	r.PUT("/category-groups/:id", h.Update)
	r.DELETE("/category-groups/:id", h.DeleteById)
}
