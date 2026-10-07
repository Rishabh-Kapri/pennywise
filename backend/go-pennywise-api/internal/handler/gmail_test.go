package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type gmailHandlerService struct {
	service.GmailService
	err    error
	called bool
}

func (s *gmailHandlerService) Pause(context.Context, model.GmailControlRequest) error {
	s.called = true
	return s.err
}
func (s *gmailHandlerService) Resume(context.Context, model.GmailControlRequest) error {
	s.called = true
	return s.err
}
func (s *gmailHandlerService) SyncNow(context.Context, model.GmailControlRequest) (*model.GmailSyncResult, error) {
	s.called = true
	return &model.GmailSyncResult{WorkflowID: "workflow", RunID: "run"}, s.err
}
func (s *gmailHandlerService) Reconnect(context.Context, model.GmailReconnectRequest) error {
	s.called = true
	return s.err
}

func gmailHandlerRequest(t *testing.T, svc *gmailHandlerService, action, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewGmailHandler(svc)
	router.POST("/gmail/reconnect", h.Reconnect)
	router.POST("/gmail/:action", h.Control)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/gmail/"+action, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func TestGmailHandlerRequiresConnectionAndReconnectCode(t *testing.T) {
	for _, input := range []struct{ action, body string }{
		{"pause", `{}`}, {"sync", `{"oauthClientType":"web"}`}, {"reconnect", `{"providerId":"google"}`},
	} {
		svc := &gmailHandlerService{}
		response := gmailHandlerRequest(t, svc, input.action, input.body)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.False(t, svc.called)
	}
}

func TestGmailHandlerReportsAcceptedSync(t *testing.T) {
	response := gmailHandlerRequest(t, &gmailHandlerService{}, "sync", `{"providerId":"google","oauthClientType":"web"}`)
	require.Equal(t, http.StatusAccepted, response.Code)
	require.JSONEq(t, `{"workflowId":"workflow","runId":"run"}`, response.Body.String())
}

func TestGmailHandlerErrorMappingDoesNotExposeUpstreamDetails(t *testing.T) {
	for _, input := range []struct {
		err     error
		status  int
		message string
	}{
		{pgx.ErrNoRows, http.StatusNotFound, "Gmail connection not found"},
		{errs.New(errs.CodeInvalidArgument, "resume before syncing"), http.StatusBadRequest, "resume before syncing"},
		{errs.Wrap(errs.CodeHTTPClientError, "ingestion is paused", errors.New("secret-token")), http.StatusBadGateway, "ingestion is paused"},
	} {
		response := gmailHandlerRequest(t, &gmailHandlerService{err: input.err}, "pause", `{"providerId":"google"}`)
		require.Equal(t, input.status, response.Code)
		require.Contains(t, response.Body.String(), input.message)
		require.NotContains(t, response.Body.String(), "secret-token")
	}
}
