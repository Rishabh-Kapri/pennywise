package authInit

import (
	"log/slog"
	"net/http"
	"os"

	"gmail-transactions/pkg/logger"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Get the oauth2 config
func getOauth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("CALLBACK_URL"),
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
	}
}

func init() {
	functions.HTTP("AuthInit", AuthInit)
	err := godotenv.Load(".env")
	if err != nil {
		logger.Fatal("error loading .env", "error", err)
	}
}

// Redirect to the auth url
func AuthInit(w http.ResponseWriter, r *http.Request) {
	authUrl := getOauth2Config().AuthCodeURL("state", oauth2.AccessTypeOffline)
	slog.Info("redirecting to url", "url", authUrl)
	http.Redirect(w, r, authUrl, http.StatusTemporaryRedirect)
}
