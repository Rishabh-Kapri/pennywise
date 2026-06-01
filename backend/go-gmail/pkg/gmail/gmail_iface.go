package gmail

import (
	"context"

	"golang.org/x/oauth2"
	gmailv1 "google.golang.org/api/gmail/v1"
)

// TokenFetcher abstracts oauth2.TokenSource so it can be mocked in tests.
// It matches the oauth2.TokenSource interface exactly.
type TokenFetcher interface {
	Token() (*oauth2.Token, error)
}

// OAuthConfig abstracts the parts of oauth2.Config used by the Service.
type OAuthConfig interface {
	// TokenSource returns a TokenFetcher for the given base token.
	// The returned value must also satisfy oauth2.TokenSource for use with
	// the real Gmail SDK; production code uses *oauth2.Config which satisfies both.
	TokenSource(ctx context.Context, t *oauth2.Token) TokenFetcher
}

// GmailAPI is the interface the Service uses to interact with the Gmail API.
type GmailAPI interface {
	StopWatch(ctx context.Context, email string) error
	SetupWatch(ctx context.Context, email string, req *gmailv1.WatchRequest) (*gmailv1.WatchResponse, error)
	ListHistory(ctx context.Context, email string, startHistoryID uint64) (*gmailv1.ListHistoryResponse, error)
	GetMessage(ctx context.Context, email string, id string) (*gmailv1.Message, error)
}

// GmailAPIFactory creates a GmailAPI from an access token.
// The production factory calls gmail.NewService with the real OAuth token source.
type GmailAPIFactory func(ctx context.Context, token *oauth2.Token, oauthConfig OAuthConfig) (GmailAPI, error)
