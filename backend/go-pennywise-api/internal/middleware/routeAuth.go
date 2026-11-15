package middleware

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
)

// ScopeByMethod is the scope check every authenticated route gets: an API key
// needs read for GET and HEAD, delete for DELETE and write for anything else.
// A route that needs more adds RequireScope.
func ScopeByMethod() gin.HandlerFunc {
	return func(c *gin.Context) {
		checkScopes(c, []sharedModel.Scope{methodScope(c.Request.Method)})
	}
}

// RequireScope stops an API key that lacks any of scopes. User sessions and
// verified internal calls aren't limited by scopes.
func RequireScope(scopes ...sharedModel.Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		checkScopes(c, scopes)
	}
}

func methodScope(method string) sharedModel.Scope {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return sharedModel.ScopeRead
	case http.MethodDelete:
		return sharedModel.ScopeDelete
	default:
		return sharedModel.ScopeWrite
	}
}

func checkScopes(c *gin.Context, scopes []sharedModel.Scope) {
	ctx := c.Request.Context()
	key := utils.APIKeyFromContext(ctx)
	if utils.VerifiedInternalFromContext(ctx) || key == nil {
		c.Next()
		return
	}
	for _, scope := range scopes {
		if !key.HasScope(scope) {
			logger.Logger(ctx).Error("not enough scopes", "required scopes", scopes, "key scopes", key.Scopes)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
	}
	c.Next()
}
