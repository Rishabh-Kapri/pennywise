package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestETagMiddleware_KeepsAnETagTheHandlerSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ETagMiddleware())
	r.PATCH("/versioned", func(c *gin.Context) {
		c.Header("ETag", `"8"`)
		c.JSON(http.StatusOK, gin.H{"version": 8})
	})
	r.GET("/hashed", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"a": 1}) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/versioned", nil))
	assert.Equal(t, `"8"`, w.Header().Get("ETag"), "the version is what the next If-Match must carry")

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hashed", nil))
	assert.Regexp(t, `^W/"[0-9a-f]{64}"$`, w.Header().Get("ETag"))
}
