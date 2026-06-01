package middleware

import (
	"math"
	"net/http"
	"strconv"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
)

func RateLimitMiddleware(rateLimitService service.RateLimitService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		log := logger.Logger(ctx)

		if utils.VerifiedInternalFromContext(ctx) {
			c.Next()
			return
		}

		key := utils.APIKeyFromContext(ctx)
		if key == nil {
			c.Next()
			return
		}

		result, err := rateLimitService.Check(ctx, key.HashedKey, int64(key.RateLimit))
		if err != nil {
			log.Error("rate limit failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "rate limit failed"})
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(key.RateLimit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
		c.Header("X-RateLimit-Reset", result.ResetAt.UTC().Format(http.TimeFormat))

		if !result.Allowed {
			retryAfter := int64(math.Ceil(result.RetryAfter.Seconds()))
			log.Debug("rate limit exceeded", "key", key, "retry_after", retryAfter, "addr", c.Request.RemoteAddr)
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
			c.Header("X-RateLimit-RetryAfter", strconv.FormatInt(retryAfter, 10))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}

		c.Next()
	}
}
