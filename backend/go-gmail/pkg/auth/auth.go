package auth

import (
	"context"

	"gmail-transactions/pkg/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Service struct {
	config *config.Config
}

func NewService(config *config.Config) *Service {
	return &Service{config: config}
}

// Get the oauth2 config
func (s *Service) GetOauth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.config.GoogleClientId,
		ClientSecret: s.config.GoogleClientSecret,
		RedirectURL:  s.config.CallbackUrl,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
	}
}

func (s *Service) GetTokenFromRefresh(refreshToken string) (*oauth2.Token, error) {
	config := s.GetOauth2Config()
	tokenSource := config.TokenSource(context.Background(), &oauth2.Token{
		RefreshToken: refreshToken,
	})
	return tokenSource.Token()
}
