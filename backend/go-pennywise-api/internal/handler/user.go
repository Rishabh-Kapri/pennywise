package handler

import (
	"net/http"
	"strings"

	"pennywise-api/internal/model"
	"pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler interface {
	Search(c *gin.Context)
	Update(c *gin.Context)
}

type userHandler struct {
	service service.UserService
}

func NewUserHandler(service service.UserService) UserHandler {
	return &userHandler{service}
}

func (h *userHandler) Search(c *gin.Context) {
	ctx := c.Request.Context()
	email := strings.TrimSpace(c.Query("email"))
	users, err := h.service.Search(ctx, email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *userHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()

	var user model.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedUser, err := h.service.Update(ctx, user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updatedUser)
}
