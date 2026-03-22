package middleware
//
// import (
// 	"net/http"
// 	"strings"
//
// 	"pennywise-api/internal/model"
// 	"pennywise-api/internal/service"
//
// 	"github.com/gin-gonic/gin"
// )
//
// // AuthMiddleware validates JWT tokens and injects user info into context
// func AuthMiddleware(authService service.AuthService) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		authHeader := c.GetHeader("Authorization")
// 		if authHeader == "" {
// 			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
// 			c.Abort()
// 			return
// 		}
//
// 		// Extract token from "Bearer <token>"
// 		parts := strings.SplitN(authHeader, " ", 2)
// 		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
// 			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
// 			c.Abort()
// 			return
// 		}
// 		tokenString := parts[1]
//
// 		// Validate token
// 		claims, err := authService.ValidateAccessToken(tokenString)
// 		if err != nil {
// 			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
// 			c.Abort()
// 			return
// 		}
//
// 		// Get user and verify token version (for logout-all-devices)
// 		user, err := authService.GetUserByID(c.Request.Context(), claims.UserID)
// 		if err != nil {
// 			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
// 			c.Abort()
// 			return
// 		}
//
// 		// Check token version matches
// 		if claims.TokenVersion != user.TokenVersion {
// 			c.JSON(http.StatusUnauthorized, gin.H{"error": "token revoked, please login again"})
// 			c.Abort()
// 			return
// 		}
//
// 		// Set user info in context
// 		c.Set("userID", claims.UserID)
// 		c.Set("userEmail", claims.Email)
// 		c.Set("user", model.AuthUserResponse{
// 			ID:      user.ID,
// 			Email:   user.Email,
// 			Name:    user.Name,
// 			Picture: user.Picture,
// 		})
//
// 		c.Next()
// 	}
// }
//
// // OptionalAuthMiddleware validates JWT if present, but doesn't require it
// func OptionalAuthMiddleware(authService service.AuthService) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		authHeader := c.GetHeader("Authorization")
// 		if authHeader == "" {
// 			c.Next()
// 			return
// 		}
//
// 		parts := strings.SplitN(authHeader, " ", 2)
// 		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
// 			c.Next()
// 			return
// 		}
// 		tokenString := parts[1]
//
// 		claims, err := authService.ValidateAccessToken(tokenString)
// 		if err != nil {
// 			c.Next()
// 			return
// 		}
//
// 		user, err := authService.GetUserByID(c.Request.Context(), claims.UserID)
// 		if err != nil {
// 			c.Next()
// 			return
// 		}
//
// 		if claims.TokenVersion == user.TokenVersion {
// 			c.Set("userID", claims.UserID)
// 			c.Set("userEmail", claims.Email)
// 			c.Set("user", model.AuthUserResponse{
// 				ID:      user.ID,
// 				Email:   user.Email,
// 				Name:    user.Name,
// 				Picture: user.Picture,
// 			})
// 		}
//
// 		c.Next()
// 	}
// }
