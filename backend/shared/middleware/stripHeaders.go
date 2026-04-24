package middleware

import "github.com/gin-gonic/gin"

func StripInternalHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Delete internal headers after handlers run.
		// Gin buffers response headers until the first Write call, so deleting
		// here (after c.Next) is safe — they won't have been sent yet.
		c.Writer.Header().Del("X-Correlation-ID")
		c.Writer.Header().Del("X-Internal-Service")
	}
}
