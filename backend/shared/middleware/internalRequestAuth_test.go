package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	authAdapter "github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

var testServices = authAdapter.NewServiceTokenAdapter(map[string]string{
	"cipher": "cipher-secret",
	"gmail":  "gmail-secret",
})

func serveInternal(t *testing.T, check func(*gin.Context), headers map[string]string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestMetadata("pennywise-api"))
	router.Use(InternalRequestAuth("api-secret", testServices))
	router.GET("/", func(c *gin.Context) {
		check(c)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusNoContent, res.Code)
}

func TestInternalRequestAuthIdentifiesCallerFromToken(t *testing.T) {
	t.Parallel()
	serveInternal(t, func(c *gin.Context) {
		ctx := c.Request.Context()
		require.True(t, utils.VerifiedInternalFromContext(ctx))
		// The token says gmail, so the header's claim of "cipher" is ignored.
		require.Equal(t, "gmail", utils.CallerServiceFromContext(ctx))
		p := auth.PrincipalFromContext(ctx)
		require.NotNil(t, p)
		require.Equal(t, auth.ServiceKind, p.Kind)
		require.Nil(t, p.UserID)
		// Outbound calls from here carry this service's own token.
		require.Equal(t, "api-secret", utils.InternalAuthTokenFromContext(ctx))
	}, map[string]string{
		utils.HeaderInternalToken: "gmail-secret",
		utils.HeaderCallerService: "cipher",
	})
}

func TestInternalRequestAuthRejectsWrongToken(t *testing.T) {
	t.Parallel()
	serveInternal(t, func(c *gin.Context) {
		ctx := c.Request.Context()
		require.True(t, utils.InternalRequestFromContext(ctx))
		require.False(t, utils.VerifiedInternalFromContext(ctx))
		require.Nil(t, auth.PrincipalFromContext(ctx))
	}, map[string]string{utils.HeaderInternalToken: "wrong-secret"})
}

func TestInternalRequestAuthRejectsLegacyHeaderAlone(t *testing.T) {
	t.Parallel()
	serveInternal(t, func(c *gin.Context) {
		require.False(t, utils.VerifiedInternalFromContext(c.Request.Context()))
	}, map[string]string{utils.HeaderInternalService: "true"})
}

func TestInternalRequestAuthRejectsOwnTokenWhenNotListed(t *testing.T) {
	t.Parallel()
	// This service's own outbound token isn't in the table: knowing it
	// doesn't let anyone call in.
	serveInternal(t, func(c *gin.Context) {
		require.False(t, utils.VerifiedInternalFromContext(c.Request.Context()))
	}, map[string]string{utils.HeaderInternalToken: "api-secret"})
}
