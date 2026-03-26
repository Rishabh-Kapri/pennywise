package handler

import (
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

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
	ctx := c.Request.Context()

	accounts, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func (h *accountHandler) Search(c *gin.Context) {
	ctx := c.Request.Context()
	name := strings.TrimSpace(c.Query("name"))
	accounts, err := h.service.Search(ctx, name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func (h *accountHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

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
