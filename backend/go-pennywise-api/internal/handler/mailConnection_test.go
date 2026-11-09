package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeMailConnections struct {
	service.MailConnectionService
	code         string
	paused       *bool
	disconnected bool
	err          error
}

func (f *fakeMailConnections) Connect(_ context.Context, code string) (service.MailConnection, error) {
	f.code = code
	return service.MailConnection{Account: "me@example.com"}, f.err
}

func TestMailConnectionConnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := func(status int) error {
		return errs.NewWith(errs.CodeHTTPClientError, "http://ingest:5180 failed", errs.WithStatus(status))
	}
	tests := []struct {
		name   string
		body   string
		err    error
		status int
	}{
		{name: "connected", body: `{"code":"abc"}`, status: http.StatusCreated},
		{name: "no code", body: `{}`, status: http.StatusBadRequest},
		{name: "no refresh token", body: `{"code":"abc"}`, err: upstream(http.StatusConflict), status: http.StatusConflict},
		{name: "refused code", body: `{"code":"abc"}`, err: upstream(http.StatusBadRequest), status: http.StatusBadRequest},
		{
			name: "ingest not configured", body: `{"code":"abc"}`,
			err: errs.New(errs.CodeMailNotConfigured, "unset"), status: http.StatusServiceUnavailable,
		},
		{name: "watch refused", body: `{"code":"abc"}`, err: upstream(http.StatusBadGateway), status: http.StatusBadGateway},
		{name: "ingest down", body: `{"code":"abc"}`, err: upstream(http.StatusInternalServerError), status: http.StatusBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.POST("/", NewMailConnectionHandler(&fakeMailConnections{err: tt.err}).Connect)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body)))
			if w.Code != tt.status {
				t.Fatalf("status = %d: %s", w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), "ingest:5180") {
				t.Fatalf("leaked the internal url: %s", w.Body)
			}
		})
	}
}

func (f *fakeMailConnections) SetPaused(
	_ context.Context, _ uuid.UUID, paused bool,
) (service.MailConnection, error) {
	f.paused = &paused
	return service.MailConnection{Account: "me@example.com"}, f.err
}

func (f *fakeMailConnections) Disconnect(context.Context, uuid.UUID) error {
	f.disconnected = true
	return f.err
}

func TestMailConnectionUpdate(t *testing.T) {
	id := uuid.New()
	t.Run("forwards an explicit false", func(t *testing.T) {
		f := &fakeMailConnections{}
		w, c := makeReq("PATCH", "/mail-connections/"+id.String(), map[string]bool{"paused": false})
		c.Params = gin.Params{{Key: "id", Value: id.String()}}
		NewMailConnectionHandler(f).Update(c)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotNil(t, f.paused)
		assert.False(t, *f.paused)
	})
	t.Run("needs paused", func(t *testing.T) {
		f := &fakeMailConnections{}
		w, c := makeReq("PATCH", "/mail-connections/"+id.String(), map[string]bool{})
		c.Params = gin.Params{{Key: "id", Value: id.String()}}
		NewMailConnectionHandler(f).Update(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Nil(t, f.paused)
	})
}

func TestMailConnectionDisconnect(t *testing.T) {
	f := &fakeMailConnections{}
	id := uuid.New()
	_, c := makeReq("DELETE", "/mail-connections/"+id.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: id.String()}}
	NewMailConnectionHandler(f).Disconnect(c)
	assert.Equal(t, http.StatusNoContent, c.Writer.Status())
	assert.True(t, f.disconnected)
}
