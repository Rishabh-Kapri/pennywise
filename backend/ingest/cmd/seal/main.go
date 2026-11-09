package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/adapters/gmail"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

func main() {
	cfg := config.Load()
	client := flag.String("client", "web", "Oauth client type the token was issued to: web | android")
	flag.Parse()

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintln(os.Stderr, "sealing token...")

	box, err := utils.NewSecretBox(cfg.CredentialsKey)
	if err != nil {
		log.Fatal(err)
	}
	sealed, err := gmail.SealCredentials(box, gmail.Credentials{
		RefreshToken:    strings.TrimSpace(string(raw)),
		OAuthClientType: model.GoogleOAuthClientType(*client),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(hex.EncodeToString(sealed))
}
