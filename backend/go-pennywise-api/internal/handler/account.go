package handler

import (
	"net/http"
	"strings"

	"pennywise-api/internal/model"
	"pennywise-api/internal/service"

	utils "pennywise-api/pkg"

	"github.com/gin-gonic/gin"
)

type AccountHandler interface {
	List(c *gin.Context)
	Search(c *gin.Context)
	Create(c *gin.Context)
}

type accountHandler struct {
	service service.AccountService
}

func NewAccountHandler(service service.AccountService) AccountHandler {
	return &accountHandler{service: service}
}

func (h *accountHandler) List(c *gin.Context) {
	ctx, err := utils.GetBudgetId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accounts, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func (h *accountHandler) Search(c *gin.Context) {
	ctx, err := utils.GetBudgetId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(c.Query("name"))
	accounts, err := h.service.Search(ctx, name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func (h *accountHandler) Create(c *gin.Context) {
	ctx, err := utils.GetBudgetId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var body model.Account
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	createdAcc, err := h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, createdAcc)
}
