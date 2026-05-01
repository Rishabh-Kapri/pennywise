package middleware

import (
	"context"
	"log"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func handleUserId(
	c *gin.Context,
	currentCtx context.Context,
	authService service.AuthService,
	userID string,
	jwtVersion float64,
) {
	log.Printf("userID: %v", userID)
	userUuid, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid user ID in token"})
		c.Abort()
		return
	}
	ctx := utils.WithUserID(currentCtx, userUuid)
	user, err := authService.GetUserById(ctx, userUuid)
	if err != nil {
		c.JSON(401, gin.H{"error": "failed to fetch user"})
		c.Abort()
		return
	}
	if user == nil {
		c.JSON(401, gin.H{"error": "user not found"})
		c.Abort()
		return
	}

	ctx = utils.WithUser(ctx, user)
	// Check token version matches
	// This is used to invalidate tokens when user logs out from all devices
	// -1 mean we have api key auth
	if jwtVersion != -1 && int(jwtVersion) != user.TokenVersion {
		c.JSON(401, gin.H{"error": "token revoked, please login again"})
		c.Abort()
		return
	}

	c.Request = c.Request.WithContext(ctx)

	c.Next()
}

func AuthMiddleware(authService service.AuthService, apiKeyService service.APIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		log := logger.Logger(ctx)
		if utils.VerifiedInternalFromContext(ctx) {
			log.Debug(
				"internal service request",
				"ip",
				c.ClientIP(),
				"caller_service",
				utils.CallerServiceFromContext(ctx),
				"origin_service",
				utils.OriginServiceFromContext(ctx),
				"path",
				c.Request.URL.Path,
			)
			c.Next()
			return
		}

		// -- API KEY AUTH --
		// we allow no api key to be passed in since we have auth header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			isValid := apiKeyService.ValidateFormat(apiKey)
			if !isValid {
				log.Error("invalid api key", "apiKey", apiKey)
				c.JSON(401, gin.H{"error": "invalid api key"})
				c.Abort()
				return
			}

			_, _, _, err := apiKeyService.ParseKey(apiKey)
			if err != nil {
				log.Error("failed to parse api key", "apiKey", apiKey)
				c.JSON(401, gin.H{"error": "invalid api key"})
				c.Abort()
				return
			}

			key, err := apiKeyService.GetByHash(ctx, apiKey)
			if err != nil {
				log.Error("failed to get api key", "key", apiKey)
				c.JSON(401, gin.H{"error": "invalid api key"})
				c.Abort()
				return
			}

			if !key.IsValid() {
				log.Error("invalid api key", "key", apiKey)
				c.JSON(401, gin.H{"error": "invalid api key"})
				c.Abort()
				return
			}
			// @TODO: check ip address
			// @TODO: check referers
			// @TODO: check scopes
			// @TODO: check rate limit
			// @TODO: update last used time

			log.Info("valid api key", "key", key)
			handleUserId(c, ctx, authService, key.UserID.String(), -1.0)
			return
		}

		// -- AUTH HEADER AUTH --
		// get the auth token from the header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			authHeader, _ = c.Cookie("access_token")
		}
		if authHeader == "" {
			if token := c.Query("token"); token != "" {
				authHeader = "Bearer " + token
			}
		}
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "unauthorized"})
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

		// validate the token
		token, err := authService.ValidateToken(ctx, tokenString)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		_, err = claims.GetAudience()
		if err != nil {
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
		jwtVersion, ok := claims["version"].(float64) // JWT stores numbers as float64
		if !ok {
			c.JSON(401, gin.H{"error": "invalid jwt version"})
			c.Abort()
			return
		}
		handleUserId(c, ctx, authService, userID, jwtVersion)
	}
}
