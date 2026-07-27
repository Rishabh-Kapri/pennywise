package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
)

type DemoHandler interface {
	LoginAsDemo(c *gin.Context)
}

type demoHandler struct {
	service service.DemoService
	config  config.Config
}

func NewDemoHandler(service service.DemoService) DemoHandler {
	return &demoHandler{service, config.Load()}
}

// LoginAsDemo handles POST /api/auth/demo.
// The route is only registered when DEMO_MODE is enabled; the in-handler
// guard is defense-in-depth against config/build mismatches.
func (h *demoHandler) LoginAsDemo(c *gin.Context) {
	if !h.config.DemoMode {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	user, accessToken, refreshToken, err := h.service.LoginAsDemo(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.SetCookie("access_token", accessToken, 3600, "/", h.config.Domain, false, true)
	c.SetCookie("refresh_token", refreshToken, 3600*24*7, "/", h.config.Domain, false, true)
	c.JSON(http.StatusOK, model.LoginResponse{
		User:         *user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes in seconds
	})
}
