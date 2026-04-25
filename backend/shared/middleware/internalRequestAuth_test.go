package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInternalRequestAuthVerifiesConfiguredToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestMetadata("pennywise-api"))
	router.Use(InternalRequestAuth("shared-secret"))

	router.GET("/", func(c *gin.Context) {
		ctx := c.Request.Context()
		require.True(t, utils.InternalRequestFromContext(ctx))
		require.True(t, utils.VerifiedInternalFromContext(ctx))
		require.Equal(t, "shared-secret", utils.InternalAuthTokenFromContext(ctx))
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderInternalService, "true")
	req.Header.Set(utils.HeaderInternalToken, "shared-secret")
	req.Header.Set(utils.HeaderCallerService, "cipher")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusNoContent, res.Code)
}

func TestInternalRequestAuthRejectsWrongToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestMetadata("pennywise-api"))
	router.Use(InternalRequestAuth("shared-secret"))

	router.GET("/", func(c *gin.Context) {
		ctx := c.Request.Context()
		require.True(t, utils.InternalRequestFromContext(ctx))
		require.False(t, utils.VerifiedInternalFromContext(ctx))
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderInternalService, "true")
	req.Header.Set(utils.HeaderInternalToken, "wrong-secret")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusNoContent, res.Code)
}

func TestInternalRequestAuthFallsBackToLegacyHeaderWithoutToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestMetadata("pennywise-api"))
	router.Use(InternalRequestAuth(""))

	router.GET("/", func(c *gin.Context) {
		ctx := c.Request.Context()
		require.True(t, utils.InternalRequestFromContext(ctx))
		require.True(t, utils.VerifiedInternalFromContext(ctx))
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderInternalService, "true")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusNoContent, res.Code)
}