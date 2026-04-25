package middleware

import (
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
)

func StripInternalHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Delete internal headers after handlers run.
		// Gin buffers response headers until the first Write call, so deleting
		// here (after c.Next) is safe — they won't have been sent yet.
		c.Writer.Header().Del(utils.HeaderCorrelationID)
		c.Writer.Header().Del(utils.HeaderCallerService)
		c.Writer.Header().Del(utils.HeaderOriginService)
		c.Writer.Header().Del(utils.HeaderInternalToken)
		c.Writer.Header().Del(utils.HeaderInternalService)
		c.Writer.Header().Del(utils.HeaderServiceName)
	}
}
