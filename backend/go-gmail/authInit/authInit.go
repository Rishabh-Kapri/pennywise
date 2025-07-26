package authInit

import (
	"log"
	"net/http"
	"os"

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
		log.Fatalf("Error while loading .env: %v", err.Error())
	}
}

// Redirect to the auth url
func AuthInit(w http.ResponseWriter, r *http.Request) {
	authUrl := getOauth2Config().AuthCodeURL("state", oauth2.AccessTypeOffline)
	log.Printf("Redirecting to url: %v", authUrl)
	http.Redirect(w, r, authUrl, http.StatusTemporaryRedirect)
}
