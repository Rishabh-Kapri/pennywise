package handler

import (
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service service.AuthService
	// cookieDomain is DOMAIN, the domain the session cookies are set for.
	cookieDomain string
}

func NewAuthHandler(service service.AuthService, cookieDomain string) *AuthHandler {
	return &AuthHandler{service: service, cookieDomain: cookieDomain}
}

// setSessionCookies sets the access cookie, and the refresh cookie when
// refreshToken isn't empty.
func setSessionCookies(c *gin.Context, domain, accessToken, refreshToken string) {
	c.SetCookie("access_token", accessToken, 3600, "/", domain, false, true)
	if refreshToken != "" {
		c.SetCookie("refresh_token", refreshToken, 3600*24*7, "/", domain, false, true)
	}
}

// loginResponse is what every login answers with.
func loginResponse(user model.AuthUserResponse, accessToken, refreshToken string) model.LoginResponse {
	return model.LoginResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(service.AccessTokenTTL.Seconds()),
	}
}

// LoginWithGoogle handles POST /api/auth/google
func (h *AuthHandler) LoginWithGoogle(c *gin.Context) {
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

	setSessionCookies(c, h.cookieDomain, accessToken, refreshToken)
	c.JSON(http.StatusOK, loginResponse(*user, accessToken, refreshToken))
}

// RefreshToken handles POST /api/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req model.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken is required"})
		return
	}

	response, err := h.service.RefreshToken(c.Request.Context(), req.RefreshToken)
	// Clients sign the user out on a 401, so it's only for a token that's
	// really no good, never for an outage.
	if errs.HasCode(err, errs.CodeAuthInvalidCredentials) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		logger.Logger(c.Request.Context()).Error("refresh token failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "could not refresh the session, try again"})
		return
	}

	setSessionCookies(c, h.cookieDomain, response.AccessToken, "")
	c.JSON(http.StatusOK, response)
}

// GetCurrentUser handles GET /api/auth/users/me.
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
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
func (h *AuthHandler) GetProviderUser(c *gin.Context) {
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

func (h *AuthHandler) Routes(public, user gin.IRouter) {
	public.POST("/auth/google", h.LoginWithGoogle)
	public.POST("/auth/refresh", h.RefreshToken)
	user.GET("/auth/users/me", h.GetCurrentUser)
	user.GET("/auth/:provider/users", h.GetProviderUser)
}
