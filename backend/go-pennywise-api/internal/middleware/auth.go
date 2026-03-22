package middleware

import (
	"strings"

	"pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// get the auth token from the header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}
		// get the user id from the token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(401, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}
		// tokenString := parts[1]
		// ctx := c.Request.Context()

		// validate the token
		// _, err := authService.ValidateToken(ctx, tokenString)
		// if err != nil {
		// 	c.JSON(401, gin.H{"error": "invalid or expired token"})
		// 	c.Abort()
		// 	return
		// }

		c.Next()
	}
}
