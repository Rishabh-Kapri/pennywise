package handler

import (
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandler interface {
	LoginWithGoogle(c *gin.Context)
	RefreshToken(c *gin.Context)
	GetCurrentUser(c *gin.Context)
	GetProviderUser(c *gin.Context)
	UpdateProviderUser(c *gin.Context)
	// Logout(c *gin.Context)
	// LogoutAll(c *gin.Context)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	user, accessToken, refreshToken, err := h.service.LoginWithGoogle(c.Request.Context(), req)
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
	logger.Logger(c.Request.Context()).Debug("refresh token response", "response", response, "error", err)
	logger.Logger(c.Request.Context()).Warn("not implemented")
	c.SetCookie("access_token", response.AccessToken, 3600, "/", h.config.Domain, false, true)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
	// c.JSON(http.StatusOK, "ok")
}

// // Logout handles POST /api/auth/logout
//
//	func (h *authHandler) Logout(c *gin.Context) {
//		// For stateless JWT, logout is handled client-side by clearing tokens
//		// This endpoint is mainly for logging/audit purposes
//		c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
//	}
//
// // LogoutAll handles POST /api/auth/logout-all
//
//	func (h *authHandler) LogoutAll(c *gin.Context) {
//		// Get user from context (set by auth middleware)
//		userID, exists := c.Get("userID")
//		if !exists {
//			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
//			return
//		}
//
//		err := h.service.LogoutAllDevices(c.Request.Context(), userID.(model.AuthUserResponse).ID)
//		if err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to logout"})
//			return
//		}
//
//		c.JSON(http.StatusOK, gin.H{"message": "logged out from all devices"})
//	}
//
// GetCurrentUser handles GET /api/auth/users/me.
func (h *authHandler) GetCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	user, err := h.service.GetCurrentUser(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

// GetProviderUser handles GET /api/auth/:provider/users?email=...
func (h *authHandler) GetProviderUser(c *gin.Context) {
	ctx := c.Request.Context()
	provider := c.Param("provider")

	switch provider {
	case "google":
		email := strings.TrimSpace(c.Query("email"))
		logger.Logger(ctx).Info("getting google user by email", "email", email)
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email query parameter is required"})
			return
		}
		user, err := h.service.GetGoogleUserByEmail(ctx, email)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, user)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider: " + provider})
	}
}

// UpdateProviderUser handles PATCH /api/auth/:provider/users
func (h *authHandler) UpdateProviderUser(c *gin.Context) {
	ctx := c.Request.Context()
	provider := c.Param("provider")

	switch provider {
	case "google":
		var req model.UpdateGmailHistoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := h.service.UpdateGmailHistoryID(
			ctx,
			req.Email,
			model.NormalizeGoogleOAuthClientType(req.OAuthClientType),
			req.GmailHistoryID,
			nil,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "history ID updated"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider: " + provider})
	}
}
