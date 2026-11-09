package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// Authenticate is the HTTP side of the auth port: it reads credentials from
// the request, asks each authenticator in order, and puts the caller in ctx
// (auth.WithPrincipal, plus utils.WithUserID / utils.WithAPIKey for code that
// reads those). Build authenticators with factory.NewAuthenticators.
//
// Credentials are read from:
//   - X-API-Key
//   - Authorization: Bearer <token>, else the access_token cookie, else the
//     ?token= query parameter (browsers can't set headers on websockets)
//
// Verified internal requests (InternalRequestAuth) pass through untouched.
//
// Responses: no credential, or a rejected one → 401; the check itself
// failing (database down) → 503, so a blip doesn't look like a logout.
func Authenticate(authenticators ...auth.Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if utils.VerifiedInternalFromContext(ctx) {
			c.Next()
			return
		}

		creds, ok := credentialsFromRequest(c)
		if !ok {
			abortJSON(c, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		for _, a := range authenticators {
			p, err := a.Authenticate(ctx, creds)
			if err != nil {
				if errs.HasCode(err, errs.CodeAuthInvalidCredentials) {
					logger.Logger(ctx).Info("authentication rejected", "err", err)
					abortJSON(c, http.StatusUnauthorized, publicMessage(err))
					return
				}
				logger.Logger(ctx).Error("authentication failed", "err", err)
				abortJSON(c, http.StatusServiceUnavailable, "authentication unavailable, try again")
				return
			}
			if p == nil {
				continue // not this authenticator's credential
			}

			ctx = auth.WithPrincipal(ctx, p)
			if p.UserID != nil {
				ctx = utils.WithUserID(ctx, *p.UserID)
			}
			if p.APIKey != nil {
				ctx = utils.WithAPIKey(ctx, p.APIKey)
			}
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		abortJSON(c, http.StatusUnauthorized, "unauthorized")
	}
}

// credentialsFromRequest reads every credential the request carries. ok is
// false when an Authorization header is present but isn't "Bearer <token>".
func credentialsFromRequest(c *gin.Context) (auth.Credentials, bool) {
	creds := auth.Credentials{APIKey: c.GetHeader(utils.HeaderAPIKey)}

	if header := c.GetHeader("Authorization"); header != "" {
		parts := strings.Split(header, " ")
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
			return creds, false
		}
		creds.BearerToken = parts[1]
		return creds, true
	}
	if token, _ := c.Cookie("access_token"); token != "" {
		creds.BearerToken = token
		return creds, true
	}
	creds.BearerToken = c.Query("token")
	return creds, true
}

// publicMessage is the error's own message, without the code prefix or the
// wrapped cause (which may come from a library and say too much).
func publicMessage(err error) string {
	var e *errs.Error
	if errors.As(err, &e) && e.Message != "" {
		return e.Message
	}
	return "unauthorized"
}

func abortJSON(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
	c.Abort()
}
