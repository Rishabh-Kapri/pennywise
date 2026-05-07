package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupRouteAuthRouter(requiredScopes ...model.Scope) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	r.Use(RouteAuthMiddleware(requiredScopes...))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestRouteAuthMiddleware_InternalRequestSkipsScopes(t *testing.T) {
	r := gin.New()
	gin.SetMode(gin.TestMode)
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	r.Use(sharedMiddleware.InternalRequestAuth("secret"))
	r.Use(RouteAuthMiddleware(model.ScopeWrite))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderInternalService, "true")
	req.Header.Set(utils.HeaderInternalToken, "secret")
	req.Header.Set(utils.HeaderCallerService, "cipher")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouteAuthMiddleware_JWTAuthSkipsScopes(t *testing.T) {
	// No API key in context → treated as JWT user → pass through
	r := setupRouteAuthRouter(model.ScopeWrite)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouteAuthMiddleware_APIKeyWithSufficientScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	// Manually inject an API key with write scope into context
	r.Use(func(c *gin.Context) {
		key := &model.APIKey{
			ID:     uuid.New(),
			Scopes: []model.Scope{model.ScopeRead, model.ScopeWrite},
		}
		ctx := utils.WithAPIKey(c.Request.Context(), key)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(RouteAuthMiddleware(model.ScopeWrite))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouteAuthMiddleware_APIKeyWithInsufficientScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	// Inject read-only API key
	r.Use(func(c *gin.Context) {
		key := &model.APIKey{
			ID:     uuid.New(),
			Scopes: []model.Scope{model.ScopeRead},
		}
		ctx := utils.WithAPIKey(c.Request.Context(), key)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(RouteAuthMiddleware(model.ScopeWrite))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
