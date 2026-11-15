package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/factory"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// internalAuth verifies X-Internal-Token "secret" as the cipher service.
func internalAuth() gin.HandlerFunc {
	services, err := factory.NewServiceAuthenticator(context.Background(),
		factory.ServiceAuthConfig{Tokens: "cipher:secret"})
	if err != nil {
		panic(err)
	}
	return sharedMiddleware.InternalRequestAuth("own", services)
}

func setupRouteAuthRouter(requiredScopes ...model.Scope) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	r.Use(RequireScope(requiredScopes...))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestRouteAuthMiddleware_InternalRequestSkipsScopes(t *testing.T) {
	r := gin.New()
	gin.SetMode(gin.TestMode)
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	r.Use(internalAuth())
	r.Use(RequireScope(model.ScopeWrite))
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
	r.Use(RequireScope(model.ScopeWrite))
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
	r.Use(RequireScope(model.ScopeWrite))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestScopeByMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serve := func(scopes []model.Scope, method string) int {
		r := gin.New()
		r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
		r.Use(func(c *gin.Context) {
			key := &model.APIKey{ID: uuid.New(), Scopes: scopes}
			c.Request = c.Request.WithContext(utils.WithAPIKey(c.Request.Context(), key))
			c.Next()
		})
		r.Use(ScopeByMethod())
		r.Handle(method, "/", func(c *gin.Context) { c.Status(http.StatusOK) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/", nil))
		return w.Code
	}

	readOnly := []model.Scope{model.ScopeRead}
	assert.Equal(t, http.StatusOK, serve(readOnly, http.MethodGet))
	assert.Equal(t, http.StatusForbidden, serve(readOnly, http.MethodPost))
	assert.Equal(t, http.StatusForbidden, serve(readOnly, http.MethodPatch))
	assert.Equal(t, http.StatusForbidden, serve(readOnly, http.MethodDelete))

	write := []model.Scope{model.ScopeWrite}
	assert.Equal(t, http.StatusOK, serve(write, http.MethodPut))
	assert.Equal(t, http.StatusForbidden, serve(write, http.MethodDelete))
	assert.Equal(t, http.StatusOK, serve([]model.Scope{model.ScopeDelete}, http.MethodDelete))
	assert.Equal(t, http.StatusOK, serve([]model.Scope{model.ScopeAdmin}, http.MethodDelete))
}
