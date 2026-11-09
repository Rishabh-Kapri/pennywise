package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
)

// RequireScopes limits API keys to routes their scopes allow. Run it after
// Authenticate. Users (JWT) and verified services aren't scoped and pass;
// an API key needs every scope listed (admin covers all), else 403.
func RequireScopes(scopes ...model.Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := auth.PrincipalFromContext(c.Request.Context())
		if p == nil || p.Kind != auth.APIKeyKind {
			c.Next()
			return
		}
		for _, s := range scopes {
			if p.APIKey == nil || !p.APIKey.HasScope(s) {
				logger.Logger(c.Request.Context()).Info("api key lacks scope", "required", scopes, "has", p.Scopes)
				abortJSON(c, http.StatusForbidden, "insufficient permissions")
				return
			}
		}
		c.Next()
	}
}
