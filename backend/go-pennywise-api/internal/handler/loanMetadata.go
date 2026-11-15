package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
)

type LoanMetadataHandler struct {
	service service.LoanMetadataService
}

func NewLoanMetadataHandler(service service.LoanMetadataService) *LoanMetadataHandler {
	return &LoanMetadataHandler{service: service}
}

func (h *LoanMetadataHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	loans, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, loans)
}

func (h *LoanMetadataHandler) GetByAccountId(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	accountId, ok := pathID(c, "accountId")
	if !ok {
		return
	}

	loan, err := h.service.GetByAccountId(ctx, accountId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "loan metadata not found"})
		return
	}
	c.JSON(http.StatusOK, loan)
}

func (h *LoanMetadataHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	var body model.LoanMetadata
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *LoanMetadataHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	accountId, ok := pathID(c, "accountId")
	if !ok {
		return
	}

	// PATCH: decode onto the stored row so fields the client leaves out keep
	// their values. The repo writes every column, and clients send only the
	// terms they edit; a blank loan_start_date empties the amortized history.
	existing, err := h.service.GetByAccountId(ctx, accountId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "loan metadata not found"})
		return
	}
	body := *existing
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.service.Update(ctx, accountId, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *LoanMetadataHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	accountId, ok := pathID(c, "accountId")
	if !ok {
		return
	}

	if err := h.service.Delete(ctx, accountId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "loan metadata deleted"})
}

func (h *LoanMetadataHandler) Routes(r gin.IRouter) {
	r.GET("/loan-metadata", h.List)
	r.GET("/loan-metadata/:accountId", h.GetByAccountId)
	r.POST("/loan-metadata", h.Create)
	r.PATCH("/loan-metadata/:accountId", h.Update)
	r.DELETE("/loan-metadata/:accountId", h.Delete)
}
