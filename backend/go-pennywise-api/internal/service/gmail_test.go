package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	tc "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	"golang.org/x/oauth2"
)

type gmailTestRepo struct {
	userID       uuid.UUID
	user         model.GoogleProviderUser
	watchUpdates int
	listError    error
}

func (r *gmailTestRepo) ListConnections(ctx context.Context, id uuid.UUID) ([]model.GoogleProviderUser, error) {
	if id != r.userID {
		return nil, pgx.ErrNoRows
	}
	return []model.GoogleProviderUser{r.user}, r.listError
}
func (r *gmailTestRepo) GetConnection(ctx context.Context, id uuid.UUID, googleID string, client model.GoogleOAuthClientType) (*model.GoogleProviderUser, error) {
	if id != r.userID || googleID != r.user.ID || client != r.user.OAuthClientType {
		return nil, pgx.ErrNoRows
	}
	copy := r.user
	return &copy, nil
}
func (r *gmailTestRepo) SetPaused(ctx context.Context, id uuid.UUID, googleID string, paused bool) error {
	r.user.GmailIngestionPaused = paused
	return nil
}
func (r *gmailTestRepo) UpdateWatch(ctx context.Context, id uuid.UUID, googleID string, client model.GoogleOAuthClientType, historyID uint64, expiry int64) error {
	r.watchUpdates++
	if r.user.GmailHistoryID == nil {
		r.user.GmailHistoryID = &historyID
	}
	r.user.ExpiryAt = &expiry
	return nil
}

type gmailTestTransport struct {
	send func(context.Context, *transport.Request) (transport.Response, error)
}

func (t gmailTestTransport) Send(ctx context.Context, req *transport.Request) (transport.Response, error) {
	return t.send(ctx, req)
}
func (t gmailTestTransport) Stream(context.Context, *transport.Request) (transport.StreamResponse, error) {
	panic("unexpected stream")
}

func gmailTestService(t *testing.T) (*gmailService, *gmailTestRepo, context.Context, model.GmailControlRequest) {
	t.Helper()
	cursor := uint64(123)
	repo := &gmailTestRepo{userID: uuid.New(), user: model.GoogleProviderUser{
		ID: "google-123", Email: "owner@example.com", OAuthClientType: model.GoogleOAuthClientTypeWeb,
		RefreshToken: "private-google-token", GmailHistoryID: &cursor,
	}}
	return &gmailService{repo: repo}, repo, utils.WithUserID(context.Background(), repo.userID), model.GmailControlRequest{
		ProviderID: repo.user.ID, OAuthClientType: repo.user.OAuthClientType,
	}
}

func TestGmailControlsRejectUnownedConnections(t *testing.T) {
	svc, repo, _, req := gmailTestService(t)
	ctx := utils.WithUserID(context.Background(), uuid.New())
	require.ErrorIs(t, svc.Pause(ctx, req), pgx.ErrNoRows)
	require.False(t, repo.user.GmailIngestionPaused)
	require.ErrorIs(t, svc.Resume(ctx, req), pgx.ErrNoRows)
	_, err := svc.SyncNow(ctx, req)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	require.ErrorIs(t, svc.Reconnect(ctx, model.GmailReconnectRequest{GmailControlRequest: req, Code: "code"}), pgx.ErrNoRows)
}

func TestGmailPausePersistsWhenGoogleIsUnavailable(t *testing.T) {
	svc, repo, ctx, req := gmailTestService(t)
	svc.client = transport.NewClient("gmail", gmailTestTransport{send: func(ctx context.Context, r *transport.Request) (transport.Response, error) {
		require.True(t, repo.user.GmailIngestionPaused, "pause must persist before stopping Google watch")
		require.Equal(t, "/api/watch", r.Path)
		require.True(t, r.Payload.(gmailSyncRequest).IsStop)
		return transport.Response{}, errors.New("Google unavailable")
	}})
	err := svc.Pause(ctx, req)
	require.ErrorContains(t, err, "ingestion is paused")
	require.True(t, repo.user.GmailIngestionPaused)
	require.EqualValues(t, 123, *repo.user.GmailHistoryID)
}

func TestGmailResumePreservesCursorAndClearsPauseAfterWatch(t *testing.T) {
	svc, repo, ctx, req := gmailTestService(t)
	repo.user.GmailIngestionPaused = true
	expiry := time.Now().Add(24 * time.Hour).UnixMilli()
	svc.client = transport.NewClient("gmail", gmailTestTransport{send: func(ctx context.Context, r *transport.Request) (transport.Response, error) {
		require.True(t, repo.user.GmailIngestionPaused)
		require.False(t, r.Payload.(gmailSyncRequest).IsStop)
		body, _ := json.Marshal(gmailSyncResponse{HistoryID: 999, Expiration: expiry})
		return transport.Response{Body: body}, nil
	}})
	require.NoError(t, svc.Resume(ctx, req))
	require.False(t, repo.user.GmailIngestionPaused)
	require.EqualValues(t, 123, *repo.user.GmailHistoryID)
	require.Equal(t, expiry, *repo.user.ExpiryAt)
}

func TestGmailResumeFailureLeavesPauseIntact(t *testing.T) {
	svc, repo, ctx, req := gmailTestService(t)
	repo.user.GmailIngestionPaused = true
	svc.client = transport.NewClient("gmail", gmailTestTransport{send: func(context.Context, *transport.Request) (transport.Response, error) {
		return transport.Response{}, errors.New("watch failed")
	}})
	require.Error(t, svc.Resume(ctx, req))
	require.True(t, repo.user.GmailIngestionPaused)
	require.Zero(t, repo.watchUpdates)
}

func TestGmailConnectionStatusDoesNotExposeCredentials(t *testing.T) {
	svc, repo, ctx, _ := gmailTestService(t)
	connections, err := svc.ListConnections(ctx)
	require.NoError(t, err)
	require.Equal(t, "watch_expired", connections[0].Status)
	data, err := json.Marshal(connections)
	require.NoError(t, err)
	require.NotContains(t, string(data), repo.user.RefreshToken)
	require.NotContains(t, string(data), "refreshToken")
	repo.user.GmailIngestionPaused = true
	require.Equal(t, "paused", gmailConnection(repo.user).Status)
	repo.user.GmailIngestionPaused = false
	repo.user.RefreshToken = ""
	require.Equal(t, "needs_reconnect", gmailConnection(repo.user).Status)
	repo.user.RefreshToken = "tok"
	expiry := time.Now().Add(time.Hour).UnixMilli()
	repo.user.ExpiryAt = &expiry
	require.Equal(t, "active", gmailConnection(repo.user).Status)
}

func TestGmailSyncRejectsPauseAndMissingCredentials(t *testing.T) {
	svc, repo, ctx, req := gmailTestService(t)
	repo.user.GmailIngestionPaused = true
	_, err := svc.SyncNow(ctx, req)
	require.ErrorContains(t, err, "resume Gmail")
	repo.user.GmailIngestionPaused = false
	repo.user.RefreshToken = ""
	_, err = svc.SyncNow(ctx, req)
	require.ErrorContains(t, err, "reconnect")
}

func TestGmailSyncStartsManualWorkflow(t *testing.T) {
	svc, _, ctx, req := gmailTestService(t)
	client := &mocks.Client{}
	run := &mocks.WorkflowRun{}
	run.On("GetID").Return("sync-id")
	run.On("GetRunID").Return("run-id")
	client.On("ExecuteWorkflow", mock.Anything, mock.MatchedBy(func(o tc.StartWorkflowOptions) bool {
		return o.TaskQueue == model.PennywiseTaskQueue && strings.HasPrefix(o.ID, "gmail-sync-")
	}), model.EmailToTransactionWorkflowName, model.EmailToTransactionWorflowInput{Email: "owner@example.com", ManualSync: true}).Return(run, nil)
	svc.temporalClient = client
	result, err := svc.SyncNow(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "sync-id", result.WorkflowID)
	client.AssertExpectations(t)
}

type gmailRoundTripper func(*http.Request) (*http.Response, error)

func (f gmailRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type gmailCredentialsRepo struct {
	repository.GoogleProviderRepository
	updated  bool
	onUpdate func(*model.GoogleProviderUser)
}

func (r *gmailCredentialsRepo) UpdateUserByGoogleIDAndClientType(ctx context.Context, id string, client model.GoogleOAuthClientType, data *model.GoogleProviderUser) error {
	r.updated = true
	if r.onUpdate != nil {
		r.onUpdate(data)
	}
	return nil
}

func gmailOAuthContext(ctx context.Context, googleID string) context.Context {
	client := &http.Client{Transport: gmailRoundTripper(func(r *http.Request) (*http.Response, error) {
		body := `{"access_token":"access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600,"scope":"https://mail.google.com/"}`
		if r.URL.Path == "/oauth2/v2/userinfo" {
			body = `{"id":"` + googleID + `","email":"owner@example.com","name":"Owner"}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	return context.WithValue(ctx, oauth2.HTTPClient, client)
}

func TestGmailReconnectRejectsDifferentGoogleAccount(t *testing.T) {
	svc, _, ctx, req := gmailTestService(t)
	credentials := &gmailCredentialsRepo{}
	svc.googleProvider = credentials
	err := svc.Reconnect(gmailOAuthContext(ctx, "another-account"), model.GmailReconnectRequest{GmailControlRequest: req, Code: "authorization-code"})
	require.ErrorContains(t, err, "choose the Google account")
	require.False(t, credentials.updated)
}

func TestGmailReconnectPreservesIntentionalPause(t *testing.T) {
	svc, repo, ctx, req := gmailTestService(t)
	repo.user.GmailIngestionPaused = true
	credentials := &gmailCredentialsRepo{onUpdate: func(data *model.GoogleProviderUser) { require.Equal(t, "new-refresh", data.RefreshToken) }}
	svc.googleProvider = credentials
	require.NoError(t, svc.Reconnect(gmailOAuthContext(ctx, repo.user.ID), model.GmailReconnectRequest{GmailControlRequest: req, Code: "authorization-code"}))
	require.True(t, credentials.updated)
	require.True(t, repo.user.GmailIngestionPaused)
	require.Zero(t, repo.watchUpdates)
}
