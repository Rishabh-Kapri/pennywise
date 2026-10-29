package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CategoryHandler interface {
	List(c *gin.Context)
	GetInflowBalance(c *gin.Context)
	Search(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	UpdateBudget(c *gin.Context)
	UpdateDetails(c *gin.Context)
	SuggestDescription(c *gin.Context)
	GetById(c *gin.Context)
	DeleteById(c *gin.Context)
}

type categoryHandler struct {
	service service.CategoryService
}

func NewCategoryHandler(service service.CategoryService) CategoryHandler {
	return &categoryHandler{service: service}
}

func (h *categoryHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	categories, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *categoryHandler) GetInflowBalance(c *gin.Context) {
	ctx := c.Request.Context()
	balance, err := h.service.GetInflowBalance(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, balance)
}

func (h *categoryHandler) Create(c *gin.Context) {
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

func (h *categoryHandler) Search(c *gin.Context) {
	ctx := c.Request.Context()
	name := strings.TrimSpace(c.Query("name"))
	logger.Logger(ctx).Debug("category search", "name", c.Query("name"))
	// if name == "" {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "Name is needed"})
	// 	return
	// }
	categories, err := h.service.Search(ctx, name)
	logger.Logger(ctx).Debug("category search result", "error", err)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *categoryHandler) GetById(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while parsing id"})
	}
	category, err := h.service.GetById(ctx, parsedId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *categoryHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while parsing id"})
		return
	}
	var body model.Category
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	err = h.service.Update(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func (h *categoryHandler) UpdateBudget(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Logger(ctx).Info("updating monthly budget")
	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}
	categoryId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while parsing id"})
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

	err = h.service.UpdateMonthlyBudget(ctx, categoryId, body.Budgeted, month)
}

func (h *categoryHandler) DeleteById(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while parsing id"})
	}
	err = h.service.DeleteById(ctx, parsedId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}

// maxCategoryTextLength bounds a category note or description; both are short
// free text, and an unbounded body is an easy way to bloat every budget fetch.
const maxCategoryTextLength = 2000

func (h *categoryHandler) UpdateDetails(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
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

func (h *categoryHandler) SuggestDescription(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
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
