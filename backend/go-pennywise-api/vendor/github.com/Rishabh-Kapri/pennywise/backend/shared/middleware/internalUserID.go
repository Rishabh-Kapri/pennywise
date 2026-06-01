package middleware

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// InternalUserIDMiddleware trusts X-User-ID only after InternalRequestAuth has
// verified the internal request token.
func InternalUserIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if !utils.VerifiedInternalFromContext(ctx) {
			c.Next()
			return
		}

		userID := c.GetHeader(utils.HeaderUserID)
		if userID == "" {
			c.Next()
			return
		}

		parsedUserID, err := uuid.Parse(userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
			c.Abort()
			return
		}

		c.Request = c.Request.WithContext(utils.WithUserID(ctx, parsedUserID))
		c.Next()
	}
}
