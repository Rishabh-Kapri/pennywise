package middleware

import (
	"crypto/subtle"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
)

// InternalRequestAuth verifies inbound internal-service headers and seeds the
// local service token onto the request context for downstream internal calls.
func InternalRequestAuth(internalAuthToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := utils.WithInternalAuthToken(c.Request.Context(), internalAuthToken)
		metadata := utils.RequestMetadataFromContext(ctx)
		logger.Logger(ctx).Info("internal request auth", "metadata", metadata)

		if metadata.InternalRequest {
			metadata.VerifiedInternal = verifyInternalRequest(
				internalAuthToken,
				c.GetHeader(utils.HeaderInternalToken),
				c.GetHeader(utils.HeaderInternalService),
			)

			if !metadata.VerifiedInternal {
				logger.Logger(ctx).Warn(
					"unverified internal request",
					"path", c.Request.URL.Path,
					"caller_service", metadata.CallerService,
					"origin_service", metadata.OriginService,
					"ip", c.ClientIP(),
				)
			}
		}

		c.Request = c.Request.WithContext(utils.WithRequestMetadata(ctx, metadata))
		c.Next()
	}
}

func verifyInternalRequest(expectedToken string, headerToken string, legacyHeader string) bool {
	if expectedToken != "" {
		return subtle.ConstantTimeCompare([]byte(expectedToken), []byte(headerToken)) == 1
	}

	return legacyHeader == "true"
}
