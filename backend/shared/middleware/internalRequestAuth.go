package middleware

import (
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
)

// InternalRequestAuth verifies inbound internal-service requests and seeds
// this service's own token onto the request context for its outbound calls.
//
// ownToken is this service's INTERNAL_AUTH_TOKEN (what it sends). services
// (factory.NewServiceAuthenticator) checks the X-Internal-Token a caller
// sent. On a match the request is VerifiedInternal, the caller's name comes
// from the token (overriding X-Caller-Service, which anyone can set), and
// the service principal is put in ctx. Anything else is an ordinary,
// unverified request that must pass user auth.
func InternalRequestAuth(ownToken string, services auth.Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := utils.WithInternalAuthToken(c.Request.Context(), ownToken)
		metadata := utils.RequestMetadataFromContext(ctx)

		if metadata.InternalRequest {
			var p *auth.Principal
			if services != nil {
				p, _ = services.Authenticate(ctx, auth.Credentials{
					ServiceToken: c.GetHeader(utils.HeaderInternalToken),
				})
			}
			metadata.VerifiedInternal = p != nil && p.Kind == auth.ServiceKind

			if metadata.VerifiedInternal {
				if p.Service != "" {
					metadata.CallerService = p.Service
				}
				ctx = auth.WithPrincipal(ctx, p)
			} else {
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
