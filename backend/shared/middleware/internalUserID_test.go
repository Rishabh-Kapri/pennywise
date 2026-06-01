package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInternalUserIDMiddlewareSetsUserIDForVerifiedInternalRequest(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestMetadata("cipher"))
	router.Use(InternalRequestAuth("shared-secret"))
	router.Use(InternalUserIDMiddleware())

	userID := uuid.New()
	router.GET("/", func(c *gin.Context) {
		actualUserID, err := utils.UserIDFromContext(c.Request.Context())
		require.NoError(t, err)
		require.Equal(t, userID, actualUserID)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderInternalService, "true")
	req.Header.Set(utils.HeaderInternalToken, "shared-secret")
	req.Header.Set(utils.HeaderUserID, userID.String())

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusNoContent, res.Code)
}

func TestInternalUserIDMiddlewareIgnoresUnverifiedUserIDHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestMetadata("cipher"))
	router.Use(InternalRequestAuth("shared-secret"))
	router.Use(InternalUserIDMiddleware())

	router.GET("/", func(c *gin.Context) {
		_, err := utils.UserIDFromContext(c.Request.Context())
		require.Error(t, err)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderUserID, uuid.NewString())

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusNoContent, res.Code)
}

func TestInternalUserIDMiddlewareRejectsInvalidVerifiedUserIDHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestMetadata("cipher"))
	router.Use(InternalRequestAuth("shared-secret"))
	router.Use(InternalUserIDMiddleware())

	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderInternalService, "true")
	req.Header.Set(utils.HeaderInternalToken, "shared-secret")
	req.Header.Set(utils.HeaderUserID, "not-a-uuid")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusBadRequest, res.Code)
}
