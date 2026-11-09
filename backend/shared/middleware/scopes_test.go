package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
)

func TestRequireScopes(t *testing.T) {
	cases := []struct {
		name string
		p    *auth.Principal
		want int
	}{
		{"user is not scoped", &auth.Principal{Kind: auth.UserKind}, http.StatusOK},
		{"service is not scoped", &auth.Principal{Kind: auth.ServiceKind}, http.StatusOK},
		{"key with write", keyPrincipal(model.ScopeWrite), http.StatusOK},
		{"key with admin", keyPrincipal(model.ScopeAdmin), http.StatusOK},
		{"read-only key", keyPrincipal(model.ScopeRead), http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.GET("/", func(c *gin.Context) {
				c.Request = c.Request.WithContext(auth.WithPrincipal(c.Request.Context(), tc.p))
				c.Next()
			}, RequireScopes(model.ScopeWrite), func(c *gin.Context) { c.Status(http.StatusOK) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func keyPrincipal(scopes ...model.Scope) *auth.Principal {
	return &auth.Principal{Kind: auth.APIKeyKind, Scopes: scopes, APIKey: &model.APIKey{Scopes: scopes}}
}
