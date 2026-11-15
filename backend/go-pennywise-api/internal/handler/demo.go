package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type DemoHandler struct {
	service      service.DemoService
	cookieDomain string
}

// NewDemoHandler serves POST /api/auth/demo. Only build it in demo mode:
// Register mounts the route only when there is one.
func NewDemoHandler(service service.DemoService, cookieDomain string) *DemoHandler {
	return &DemoHandler{service: service, cookieDomain: cookieDomain}
}

// LoginAsDemo handles POST /api/auth/demo.
func (h *DemoHandler) LoginAsDemo(c *gin.Context) {
	user, accessToken, refreshToken, err := h.service.LoginAsDemo(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	setSessionCookies(c, h.cookieDomain, accessToken, refreshToken)
	c.JSON(http.StatusOK, loginResponse(*user, accessToken, refreshToken))
}

func (h *DemoHandler) Routes(r gin.IRouter) {
	r.POST("/auth/demo", h.LoginAsDemo)
}
