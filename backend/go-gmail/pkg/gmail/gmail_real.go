package gmail

import (
	"context"

	"golang.org/x/oauth2"
	gmailv1 "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// WrapOAuthConfig wraps a *oauth2.Config so it satisfies the OAuthConfig interface.
// Call sites in runner and temporal packages use this to adapt their *oauth2.Config.
func WrapOAuthConfig(cfg *oauth2.Config) OAuthConfig {
	return &realOAuthConfig{cfg: cfg}
}

// realOAuthConfig wraps *oauth2.Config to satisfy OAuthConfig.
type realOAuthConfig struct {
	cfg *oauth2.Config
}

func (r *realOAuthConfig) TokenSource(ctx context.Context, t *oauth2.Token) TokenFetcher {
	// oauth2.TokenSource satisfies TokenFetcher (both have Token() (*Token, error))
	return r.cfg.TokenSource(ctx, t)
}

// realGmailAPI is the production GmailAPI backed by the real Gmail SDK.
type realGmailAPI struct {
	svc *gmailv1.Service
}

// newRealGmailAPI is the production GmailAPIFactory implementation.
func newRealGmailAPI(ctx context.Context, token *oauth2.Token, oauthConfig OAuthConfig) (GmailAPI, error) {
	// The TokenFetcher returned by realOAuthConfig is an oauth2.TokenSource,
	// so the type assertion is safe in production.
	ts := oauthConfig.TokenSource(ctx, token)
	oauthTS, ok := ts.(oauth2.TokenSource)
	if !ok {
		// Fallback: wrap the TokenFetcher as an oauth2.TokenSource.
		oauthTS = &tokenFetcherAdapter{tf: ts}
	}
	svc, err := gmailv1.NewService(ctx, option.WithTokenSource(oauthTS))
	if err != nil {
		return nil, err
	}
	return &realGmailAPI{svc: svc}, nil
}

// tokenFetcherAdapter adapts a TokenFetcher to oauth2.TokenSource.
type tokenFetcherAdapter struct {
	tf TokenFetcher
}

func (a *tokenFetcherAdapter) Token() (*oauth2.Token, error) {
	return a.tf.Token()
}

func (g *realGmailAPI) StopWatch(ctx context.Context, email string) error {
	return g.svc.Users.Stop(email).Do()
}

func (g *realGmailAPI) SetupWatch(ctx context.Context, email string, req *gmailv1.WatchRequest) (*gmailv1.WatchResponse, error) {
	return gmailv1.NewUsersService(g.svc).Watch(email, req).Do()
}

func (g *realGmailAPI) ListHistory(ctx context.Context, email string, startHistoryID uint64) (*gmailv1.ListHistoryResponse, error) {
	return g.svc.Users.History.List(email).StartHistoryId(startHistoryID).Do()
}

func (g *realGmailAPI) GetMessage(ctx context.Context, email string, id string) (*gmailv1.Message, error) {
	return g.svc.Users.Messages.Get(email, id).Do()
}
