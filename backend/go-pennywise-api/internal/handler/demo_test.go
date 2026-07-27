package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ─────────────────────────────────────────────────────────────────────────────
// DemoHandler
// ─────────────────────────────────────────────────────────────────────────────

type mockDemoService struct{ mock.Mock }

func (m *mockDemoService) LoginAsDemo(ctx context.Context) (*model.AuthUserResponse, string, string, error) {
	args := m.Called(ctx)
	var user *model.AuthUserResponse
	if v := args.Get(0); v != nil {
		user = v.(*model.AuthUserResponse)
	}
	return user, args.String(1), args.String(2), args.Error(3)
}

func TestDemoHandler_LoginAsDemo(t *testing.T) {
	t.Run("returns_404_when_demo_mode_disabled", func(t *testing.T) {
		t.Setenv("DEMO_MODE", "false")
		svc := &mockDemoService{}
		w, c := makeReq("POST", "/auth/demo", nil)
		NewDemoHandler(svc).LoginAsDemo(c)
		assert.Equal(t, http.StatusNotFound, w.Code)
		svc.AssertNotCalled(t, "LoginAsDemo")
	})

	t.Run("returns_tokens_when_enabled", func(t *testing.T) {
		t.Setenv("DEMO_MODE", "true")
		svc := &mockDemoService{}
		svc.On("LoginAsDemo", mock.Anything).Return(
			&model.AuthUserResponse{ID: uuid.New(), Email: "demo@pennywise.local", Name: "Demo User"},
			"access-token", "refresh-token", nil,
		)
		w, c := makeReq("POST", "/auth/demo", nil)
		NewDemoHandler(svc).LoginAsDemo(c)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp model.LoginResponse
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "access-token", resp.AccessToken)
		assert.Equal(t, "refresh-token", resp.RefreshToken)
		assert.Equal(t, "demo@pennywise.local", resp.User.Email)
		assert.Equal(t, 900, resp.ExpiresIn)
		svc.AssertExpectations(t)
	})

	t.Run("service_error_returns_500", func(t *testing.T) {
		t.Setenv("DEMO_MODE", "true")
		svc := &mockDemoService{}
		svc.On("LoginAsDemo", mock.Anything).Return(nil, "", "", assert.AnError)
		w, c := makeReq("POST", "/auth/demo", nil)
		NewDemoHandler(svc).LoginAsDemo(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
