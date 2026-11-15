package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStripInternalHeadersKeepsThemOffTheResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(StripInternalHeaders())
	setAll := func(c *gin.Context) {
		for _, h := range internalResponseHeaders {
			c.Header(h, "secret")
		}
		c.Header("X-Public", "kept")
	}
	r.GET("/json", func(c *gin.Context) { setAll(c); c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/status", func(c *gin.Context) { setAll(c); c.Status(http.StatusNoContent) })
	r.GET("/empty", func(c *gin.Context) { setAll(c) })
	r.GET("/string", func(c *gin.Context) { setAll(c); c.String(http.StatusOK, "hi") })

	for _, path := range []string{"/json", "/status", "/string", "/empty"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			for _, h := range internalResponseHeaders {
				require.Empty(t, w.Header().Values(h), h)
			}
			require.Equal(t, "kept", w.Header().Get("X-Public"))
		})
	}
}
