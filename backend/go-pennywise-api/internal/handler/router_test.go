package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRegister_DemoLoginOnlyInDemoMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pass := func(c *gin.Context) { c.Next() }
	mw := Middleware{Authenticate: pass, RateLimit: pass, Budget: pass}
	demoRoute := func(h Handlers) int {
		r := gin.New()
		Register(r, h, mw)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/demo", nil))
		return w.Code
	}

	assert.Equal(t, http.StatusNotFound, demoRoute(Handlers{}))

	svc := &mockDemoService{}
	svc.On("LoginAsDemo", mock.Anything).Return(nil, "", "", assert.AnError)
	assert.Equal(t, http.StatusInternalServerError, demoRoute(Handlers{Demo: NewDemoHandler(svc, "")}))
}
