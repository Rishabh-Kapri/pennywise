package service

import (
	"context"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
	tc "go.temporal.io/sdk/client"
)

type GmailService interface {
	ListConnections(ctx context.Context) ([]model.GmailConnection, error)
	Pause(ctx context.Context, req model.GmailControlRequest) error
	Resume(ctx context.Context, req model.GmailControlRequest) error
	SyncNow(ctx context.Context, req model.GmailControlRequest) (*model.GmailSyncResult, error)
	Reconnect(ctx context.Context, req model.GmailReconnectRequest) error
}

type gmailService struct {
	repo           repository.GmailRepository
	googleProvider repository.GoogleProviderRepository
	client         *transport.Client
	temporalClient tc.Client
	config         config.Config
}

func NewGmailService(repo repository.GmailRepository, googleProvider repository.GoogleProviderRepository, client *transport.Client, temporalClient tc.Client) GmailService {
	return &gmailService{repo: repo, googleProvider: googleProvider, client: client, temporalClient: temporalClient, config: config.Load()}
}

func gmailConnection(user model.GoogleProviderUser) model.GmailConnection {
	connected := user.RefreshToken != ""
	status := "needs_reconnect"
	if connected {
		status = "watch_expired"
		if user.ExpiryAt != nil && *user.ExpiryAt > time.Now().UnixMilli() {
			status = "active"
		}
	}
	if user.GmailIngestionPaused {
		status = "paused"
	}
	return model.GmailConnection{
		ProviderID: user.ID, OAuthClientType: user.OAuthClientType, Email: user.Email,
		Name: user.Name, Picture: user.Picture, Paused: user.GmailIngestionPaused,
		Connected: connected, Status: status, LastGmailSync: user.LastGmailSync, WatchExpiresAt: user.ExpiryAt,
	}
}

func (s *gmailService) ListConnections(ctx context.Context) ([]model.GmailConnection, error) {
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	users, err := s.repo.ListConnections(ctx, userID)
	if err != nil {
		return nil, err
	}
	connections := make([]model.GmailConnection, 0, len(users))
	for _, user := range users {
		connections = append(connections, gmailConnection(user))
	}
	return connections, nil
}

func (s *gmailService) connection(ctx context.Context, req model.GmailControlRequest) (*model.GoogleProviderUser, uuid.UUID, error) {
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if strings.TrimSpace(req.ProviderID) == "" || (req.OAuthClientType != "" && req.OAuthClientType != model.GoogleOAuthClientTypeWeb && req.OAuthClientType != model.GoogleOAuthClientTypeAndroid) {
		return nil, userID, errs.New(errs.CodeInvalidArgument, "invalid Gmail connection")
	}
	user, err := s.repo.GetConnection(ctx, userID, req.ProviderID, model.NormalizeGoogleOAuthClientType(req.OAuthClientType))
	if err != nil {
		return nil, userID, err
	}
	if user.Email == demoUserEmail {
		return nil, userID, errs.New(errs.CodeInvalidArgument, "Gmail controls are unavailable for the demo account")
	}
	return user, userID, nil
}

func (s *gmailService) watch(ctx context.Context, user *model.GoogleProviderUser, stop bool) (gmailSyncResponse, error) {
	if user.RefreshToken == "" {
		return gmailSyncResponse{}, errs.New(errs.CodeInvalidArgument, "reconnect Gmail to grant mailbox access")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	res, err := transport.Post[gmailSyncResponse](ctx, s.client, "/api/watch", nil, gmailSyncRequest{
		Email: user.Email, RefreshToken: user.RefreshToken, OAuthClientType: user.OAuthClientType, IsStop: stop,
	})
	if err != nil {
		return res, errs.Wrap(errs.CodeHTTPClientError, "Gmail could not update the connection; try again or reconnect Gmail", err)
	}
	if !stop && (res.HistoryID == 0 || res.Expiration <= time.Now().UnixMilli()) {
		return res, errs.New(errs.CodeHTTPClientError, "Gmail returned an invalid watch state")
	}
	return res, nil
}

func (s *gmailService) Pause(ctx context.Context, req model.GmailControlRequest) error {
	user, userID, err := s.connection(ctx, req)
	if err != nil {
		return err
	}
	// Persist first so queued pushes are ignored even if Google is unavailable.
	if err := s.repo.SetPaused(ctx, userID, user.ID, true); err != nil {
		return err
	}
	if user.RefreshToken == "" {
		return nil
	}
	if _, err := s.watch(ctx, user, true); err != nil {
		return errs.Wrap(errs.CodeHTTPClientError, "ingestion is paused, but Gmail could not stop its watch", err)
	}
	return nil
}

func (s *gmailService) Resume(ctx context.Context, req model.GmailControlRequest) error {
	user, userID, err := s.connection(ctx, req)
	if err != nil {
		return err
	}
	res, err := s.watch(ctx, user, false)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateWatch(ctx, userID, user.ID, user.OAuthClientType, res.HistoryID, res.Expiration); err != nil {
		return err
	}
	return s.repo.SetPaused(ctx, userID, user.ID, false)
}

func (s *gmailService) SyncNow(ctx context.Context, req model.GmailControlRequest) (*model.GmailSyncResult, error) {
	user, _, err := s.connection(ctx, req)
	if err != nil {
		return nil, err
	}
	if user.GmailIngestionPaused {
		return nil, errs.New(errs.CodeInvalidArgument, "resume Gmail ingestion before syncing")
	}
	if user.RefreshToken == "" || user.GmailHistoryID == nil {
		return nil, errs.New(errs.CodeInvalidArgument, "reconnect or renew the Gmail watch before syncing")
	}
	if s.temporalClient == nil {
		return nil, errs.New(errs.CodeInternalError, "Gmail sync is unavailable because Temporal is not configured")
	}
	// The workflow looks up the mailbox's current ingestion cursor and budget.
	// Manual sync does not replace that cursor with a watch's latest history ID.
	run, err := s.temporalClient.ExecuteWorkflow(ctx, tc.StartWorkflowOptions{
		ID: "gmail-sync-" + uuid.NewString(), TaskQueue: model.PennywiseTaskQueue,
	}, model.EmailToTransactionWorkflowName, model.EmailToTransactionWorflowInput{Email: user.Email, ManualSync: true})
	if err != nil {
		return nil, err
	}
	return &model.GmailSyncResult{WorkflowID: run.GetID(), RunID: run.GetRunID()}, nil
}

func (s *gmailService) Reconnect(ctx context.Context, req model.GmailReconnectRequest) error {
	user, _, err := s.connection(ctx, req.GmailControlRequest)
	if err != nil {
		return err
	}
	if user.OAuthClientType != model.GoogleOAuthClientTypeWeb {
		return errs.New(errs.CodeInvalidArgument, "reconnect this Google connection from the Android app")
	}
	helper := &authService{config: s.config}
	oauthConfig := helper.getOauth2Config()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	token, err := oauthConfig.Exchange(ctx, req.Code)
	if err != nil {
		return errs.Wrap(errs.CodeInvalidArgument, "Google authorization failed; try reconnecting again", err)
	}
	profile, err := helper.fetchGoogleUser(ctx, oauthConfig, token)
	if err != nil {
		return err
	}
	if profile.ID != user.ID {
		return errs.New(errs.CodeInvalidArgument, "choose the Google account already connected to Pennywise")
	}
	if scope, ok := token.Extra("scope").(string); ok && !strings.Contains(scope, "https://mail.google.com/") && !strings.Contains(scope, "https://www.googleapis.com/auth/gmail.readonly") {
		return errs.New(errs.CodeInvalidArgument, "grant Gmail access to reconnect this mailbox")
	}
	if token.RefreshToken == "" && user.RefreshToken == "" {
		return errs.New(errs.CodeInvalidArgument, "Google did not grant offline access; remove Pennywise from your Google account permissions and reconnect")
	}
	if err := s.googleProvider.UpdateUserByGoogleIDAndClientType(ctx, user.ID, user.OAuthClientType, &model.GoogleProviderUser{
		Name: profile.Name, Picture: profile.Picture, RefreshToken: token.RefreshToken,
	}); err != nil {
		return err
	}
	if token.RefreshToken != "" {
		user.RefreshToken = token.RefreshToken
	}
	// Reconnecting credentials keeps an intentional pause in place.
	if user.GmailIngestionPaused {
		return nil
	}
	return s.Resume(ctx, req.GmailControlRequest)
}
