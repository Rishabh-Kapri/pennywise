package auth

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Service struct {
	config *config.Config
}

func NewService(config *config.Config) *Service {
	return &Service{config: config}
}

// Get the web OAuth2 config.
func (s *Service) GetOauth2Config() *oauth2.Config {
	return s.GetOauth2ConfigForClientType(model.GoogleOAuthClientTypeWeb)
}

func (s *Service) GetOauth2ConfigForClientType(clientType model.GoogleOAuthClientType) *oauth2.Config {
	clientType = model.NormalizeGoogleOAuthClientType(clientType)
	if clientType == model.GoogleOAuthClientTypeAndroid {
		return &oauth2.Config{
			ClientID: s.config.GoogleAndroidClientId,
			Endpoint: google.Endpoint,
			Scopes:   []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
		}
	}

	return &oauth2.Config{
		ClientID:     s.config.GoogleClientId,
		ClientSecret: s.config.GoogleClientSecret,
		RedirectURL:  s.config.CallbackUrl,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
	}
}

func (s *Service) GetTokenFromRefresh(refreshToken string, clientType model.GoogleOAuthClientType) (*oauth2.Token, error) {
	config := s.GetOauth2ConfigForClientType(clientType)
	tokenSource := config.TokenSource(context.Background(), &oauth2.Token{
		RefreshToken: refreshToken,
	})
	return tokenSource.Token()
}
