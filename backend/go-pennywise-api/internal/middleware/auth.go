package middleware

import (
	"context"
	"strings"

	"pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
		// check if the header is in the format "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(401, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}
		tokenString := parts[1]
		ctx := c.Request.Context()

		// validate the token
		token, err := authService.ValidateToken(ctx, tokenString)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		email := claims["email"].(string)
		if email == "" {
			c.JSON(401, gin.H{"error": "email claim missing in token"})
			c.Abort()
			return
		}

		// fetch user info and set it in the context (optional, can be used in handlers)
		userID, err := claims.GetSubject()
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}
		// userUuid, err := uuid.Parse(userID)
		// if err != nil {
		// 	c.JSON(401, gin.H{"error": "invalid user ID in token"})
		// 	c.Abort()
		// 	return
		// }

		ctx = context.WithValue(c.Request.Context(), "userID", userID)
		ctx = context.WithValue(ctx, "userEmail", email)
		c.Request = c.Request.WithContext(ctx)

		// Optionally, you can fetch the full user info and set it in the context
		// user, err := authService.GetUserById(ctx, userUuid)
		// if err != nil {
		// 	c.JSON(401, gin.H{"error": "failed to fetch user"})
		// 	c.Abort()
		// 	return
		// }
		// c.Set("user", user)

		c.Next()
	}
}
