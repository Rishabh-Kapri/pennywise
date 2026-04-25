package middleware

import (
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
)

// RequestMetadata normalizes inbound request metadata into context so downstream
// middleware, logs, and outbound clients share the same correlation and caller fields.
func RequestMetadata(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current := utils.RequestMetadataFromContext(c.Request.Context())
		inbound := utils.RequestMetadataFromHeaders(c.Request.Header)
		logger.Logger(c.Request.Context()).Info("request metadata", "inbound", inbound, "current", current)

		metadata := current
		if inbound.CorrelationID != "" {
			metadata.CorrelationID = inbound.CorrelationID
		}
		if inbound.CallerService != "" {
			metadata.CallerService = inbound.CallerService
		}
		if inbound.OriginService != "" {
			metadata.OriginService = inbound.OriginService
		}
		metadata.InternalRequest = metadata.InternalRequest || inbound.InternalRequest
		metadata.LocalService = serviceName

		// Do not trust inbound budget/user headers at the ingress layer.
		// Budget and user context are established by dedicated middleware/auth flows.
		if metadata.CorrelationID == "" {
			metadata.CorrelationID = utils.NewCorrelationID()
		}

		c.Request = c.Request.WithContext(utils.WithRequestMetadata(c.Request.Context(), metadata))
		c.Next()
	}
}

