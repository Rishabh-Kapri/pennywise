package model

import "time"

// GmailConnection exposes watch state without exposing Google credentials.
type GmailConnection struct {
	ProviderID      string                `json:"providerId"`
	OAuthClientType GoogleOAuthClientType `json:"oauthClientType"`
	Email           string                `json:"email"`
	Name            string                `json:"name"`
	Picture         string                `json:"picture,omitempty"`
	Paused          bool                  `json:"paused"`
	Connected       bool                  `json:"connected"`
	Status          string                `json:"status"`
	LastGmailSync   *time.Time            `json:"lastGmailSync,omitempty"`
	WatchExpiresAt  *int64                `json:"watchExpiresAt,omitempty"`
}

type GmailControlRequest struct {
	ProviderID      string                `json:"providerId" binding:"required"`
	OAuthClientType GoogleOAuthClientType `json:"oauthClientType,omitempty"`
}

type GmailReconnectRequest struct {
	GmailControlRequest
	Code string `json:"code" binding:"required"`
}

type GmailSyncResult struct {
	WorkflowID string `json:"workflowId"`
	RunID      string `json:"runId"`
}
