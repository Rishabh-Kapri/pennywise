package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler interface {
	LoginWithGoogle(c *gin.Context)
	RefreshToken(c *gin.Context)
	// Logout(c *gin.Context)
	// LogoutAll(c *gin.Context)
	// GetCurrentUser(c *gin.Context)
}

type authHandler struct {
	service service.AuthService
	config  config.Config
}

func NewAuthHandler(service service.AuthService) AuthHandler {
	return &authHandler{service, config.Load()}
}

// LoginWithGoogle handles POST /api/auth/google
func (h *authHandler) LoginWithGoogle(c *gin.Context) {
	var req model.GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "credential is required"})
		return
	}

	user, accessToken, refreshToken, err := h.service.LoginWithGoogle(c.Request.Context(), req.Credential)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.SetCookie("access_token", accessToken, 3600, "/", h.config.Domain, false, true)
	c.SetCookie("refresh_token", refreshToken, 3600*24*7, "/", h.config.Domain, false, true)
	c.JSON(http.StatusOK, model.LoginResponse{
		User: model.AuthUserResponse{
			ID:      user.ID,
			Email:   user.Email,
			Name:    user.Name,
			Picture: user.Picture,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes in seconds
	})
}

// RefreshToken handles POST /api/auth/refresh
func (h *authHandler) RefreshToken(c *gin.Context) {
	var req model.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken is required"})
		return
	}

	response, err := h.service.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, response)
}
//
// // Logout handles POST /api/auth/logout
// func (h *authHandler) Logout(c *gin.Context) {
// 	// For stateless JWT, logout is handled client-side by clearing tokens
// 	// This endpoint is mainly for logging/audit purposes
// 	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
// }
//
// // LogoutAll handles POST /api/auth/logout-all
// func (h *authHandler) LogoutAll(c *gin.Context) {
// 	// Get user from context (set by auth middleware)
// 	userID, exists := c.Get("userID")
// 	if !exists {
// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
// 		return
// 	}
//
// 	err := h.service.LogoutAllDevices(c.Request.Context(), userID.(model.AuthUserResponse).ID)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to logout"})
// 		return
// 	}
//
// 	c.JSON(http.StatusOK, gin.H{"message": "logged out from all devices"})
// }
//
// // GetCurrentUser handles GET /api/auth/me
// func (h *authHandler) GetCurrentUser(c *gin.Context) {
// 	user, exists := c.Get("user")
// 	if !exists {
// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
// 		return
// 	}
//
// 	c.JSON(http.StatusOK, user)
// }
