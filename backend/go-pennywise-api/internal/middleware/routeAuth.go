package middleware

import (
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
)

/* Middleware to handle the route authentication
* Fetches the api key from context and checks against requiredScopes
 */
func RouteAuthMiddleware(requiredScopes ...sharedModel.Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		log := logger.Logger(ctx)

		// skip for internal service calls
		if utils.VerifiedInternalFromContext(ctx) {
			c.Next()
			return
		}

		key := utils.APIKeyFromContext(ctx)
		// jwt auth gets normal access
		if key == nil {
			c.Next()
			return
		}

		// check scopes
		for _, requiredScope := range requiredScopes {
			if !key.HasScope(requiredScope) {
				log.Error("not enough scopes", "required scopes", requiredScopes, "key scopes", key.Scopes)
				c.JSON(403, gin.H{"error": "insufficient permissions"})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
