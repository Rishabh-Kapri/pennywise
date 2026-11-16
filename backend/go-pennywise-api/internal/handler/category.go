package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
)

type CategoryHandler struct {
	service service.CategoryService
}

func NewCategoryHandler(service service.CategoryService) *CategoryHandler {
	return &CategoryHandler{service: service}
}

func (h *CategoryHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	categories, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *CategoryHandler) GetInflowBalance(c *gin.Context) {
	ctx := c.Request.Context()
	balance, err := h.service.GetInflowBalance(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, balance)
}

func (h *CategoryHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var body model.Category
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	createdCat, err := h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, createdCat)
}

func (h *CategoryHandler) Search(c *gin.Context) {
	ctx := c.Request.Context()
	name := strings.TrimSpace(c.Query("name"))
	logger.Logger(ctx).Debug("category search", "name", c.Query("name"))
	categories, err := h.service.Search(ctx, name)
	logger.Logger(ctx).Debug("category search result", "error", err)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *CategoryHandler) GetById(c *gin.Context) {
	ctx := c.Request.Context()

	parsedId, ok := pathID(c, "id")
	if !ok {
		return
	}
	category, err := h.service.GetById(ctx, parsedId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *CategoryHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	parsedId, ok := pathID(c, "id")
	if !ok {
		return
	}
	var body model.Category
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	err := h.service.Update(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func (h *CategoryHandler) UpdateBudget(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Logger(ctx).Info("updating monthly budget")
	categoryId, ok := pathID(c, "id")
	if !ok {
		return
	}
	month, ok := c.Params.Get("month")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Month is needed"})
		return
	}
	logger.Logger(ctx).Info("updating monthly budget", "categoryId", categoryId, "month", month)
	var body model.MonthlyBudget
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateMonthlyBudget(ctx, categoryId, body.Budgeted, month); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (h *CategoryHandler) DeleteById(c *gin.Context) {
	ctx := c.Request.Context()

	parsedId, ok := pathID(c, "id")
	if !ok {
		return
	}
	err := h.service.DeleteById(ctx, parsedId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}

// maxCategoryTextLength bounds a category note or description; both are short
// free text, and an unbounded body is an easy way to bloat every budget fetch.
const maxCategoryTextLength = 2000

func (h *CategoryHandler) UpdateDetails(c *gin.Context) {
	ctx := c.Request.Context()
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var body model.CategoryDetailsUpdate
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Note == nil && body.Description == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note or description is required"})
		return
	}
	for _, field := range []*string{body.Note, body.Description} {
		if field == nil {
			continue
		}
		*field = strings.TrimSpace(*field)
		if len(*field) > maxCategoryTextLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": "text is too long"})
			return
		}
	}

	if err := h.service.UpdateDetails(ctx, id, body); err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, body)
}

func (h *CategoryHandler) SuggestDescription(c *gin.Context) {
	ctx := c.Request.Context()
	id, ok := pathID(c, "id")
	if !ok {
		return
	}

	description, err := h.service.SuggestDescription(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCategoryNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrDescriptionUnavailable):
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"description": description})
}

func (h *CategoryHandler) Routes(r gin.IRouter) {
	r.POST("/categories", h.Create)
	r.GET("/categories", h.List)
	r.GET("/categories/inflow", h.GetInflowBalance)
	r.PATCH("/categories/:id/:month", h.UpdateBudget)
	r.GET("/categories/search", h.Search)
	r.GET("/categories/:id", h.GetById)
	r.PUT("/categories/:id", h.Update)
	r.PUT("/categories/:id/details", h.UpdateDetails)
	r.POST("/categories/:id/description/suggest", h.SuggestDescription)
	r.DELETE("/categories/:id", h.DeleteById)
}
