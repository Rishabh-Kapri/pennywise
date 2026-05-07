package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock RateLimitService
// ─────────────────────────────────────────────────────────────────────────────

type mockRateLimitService struct{ mock.Mock }

func (m *mockRateLimitService) Check(ctx context.Context, keyHash string, limit int64) (*service.RateLimitResult, error) {
	args := m.Called(ctx, keyHash, limit)
	if v := args.Get(0); v != nil {
		return v.(*service.RateLimitResult), args.Error(1)
	}
	return nil, args.Error(1)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper
// ─────────────────────────────────────────────────────────────────────────────

func setupRateLimitRouter(rl service.RateLimitService, injectKey *model.APIKey) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	if injectKey != nil {
		r.Use(func(c *gin.Context) {
			ctx := utils.WithAPIKey(c.Request.Context(), injectKey)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		})
	}
	r.Use(RateLimitMiddleware(rl))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestRateLimitMiddleware_InternalRequestBypasses(t *testing.T) {
	rl := &mockRateLimitService{}

	r := gin.New()
	gin.SetMode(gin.TestMode)
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	r.Use(sharedMiddleware.InternalRequestAuth("secret"))
	r.Use(RateLimitMiddleware(rl))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderInternalService, "true")
	req.Header.Set(utils.HeaderInternalToken, "secret")
	req.Header.Set(utils.HeaderCallerService, "cipher")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	rl.AssertNotCalled(t, "Check")
}

func TestRateLimitMiddleware_NoAPIKey_Passes(t *testing.T) {
	rl := &mockRateLimitService{}
	// No API key in context → skip rate limit
	r := setupRateLimitRouter(rl, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	rl.AssertNotCalled(t, "Check")
}

func TestRateLimitMiddleware_WithinLimit_Passes(t *testing.T) {
	rl := &mockRateLimitService{}
	key := &model.APIKey{
		ID:        uuid.New(),
		HashedKey: "hashed-key",
		RateLimit: 100,
	}
	rl.On("Check", mock.Anything, "hashed-key", int64(100)).Return(&service.RateLimitResult{
		Allowed:    true,
		Remaining:  99,
		ResetAt:    time.Now().Add(time.Minute),
		RetryAfter: 0,
	}, nil)

	r := setupRateLimitRouter(rl, key)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "100", w.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "99", w.Header().Get("X-RateLimit-Remaining"))
	rl.AssertExpectations(t)
}

func TestRateLimitMiddleware_ExceededLimit_Returns429(t *testing.T) {
	rl := &mockRateLimitService{}
	key := &model.APIKey{
		ID:        uuid.New(),
		HashedKey: "hashed-key",
		RateLimit: 10,
	}
	rl.On("Check", mock.Anything, "hashed-key", int64(10)).Return(&service.RateLimitResult{
		Allowed:    false,
		Remaining:  0,
		ResetAt:    time.Now().Add(time.Minute),
		RetryAfter: 30 * time.Second,
	}, nil)

	r := setupRateLimitRouter(rl, key)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "30", w.Header().Get("Retry-After"))
	rl.AssertExpectations(t)
}

func TestRateLimitMiddleware_ServiceError_Returns500(t *testing.T) {
	rl := &mockRateLimitService{}
	key := &model.APIKey{
		ID:        uuid.New(),
		HashedKey: "hashed-key",
		RateLimit: 50,
	}
	rl.On("Check", mock.Anything, "hashed-key", int64(50)).Return(nil, assert.AnError)

	r := setupRateLimitRouter(rl, key)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	rl.AssertExpectations(t)
}
