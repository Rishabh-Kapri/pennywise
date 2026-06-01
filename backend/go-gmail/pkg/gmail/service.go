package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmail "google.golang.org/api/gmail/v1"
)

type Service struct {
	config          *config.Config
	gmailAPIFactory GmailAPIFactory
	oauthConfig     OAuthConfig
}

func getOauth2Config(clientType model.GoogleOAuthClientType) OAuthConfig {
	clientType = model.NormalizeGoogleOAuthClientType(clientType)
	if clientType == model.GoogleOAuthClientTypeAndroid {
		return &realOAuthConfig{
			cfg: &oauth2.Config{
				ClientID: os.Getenv("GOOGLE_ANDROID_CLIENT_ID"),
				Endpoint: google.Endpoint,
				Scopes:   []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
			},
		}
	}

	return &realOAuthConfig{
		cfg: &oauth2.Config{
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  "postmessage",
			Endpoint:     google.Endpoint,
			Scopes:       []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
		},
	}
}

func NewService() *Service {
	cfg := config.LoadConfig()
	return &Service{
		config:          cfg,
		gmailAPIFactory: newRealGmailAPI,
		oauthConfig:     getOauth2Config(model.GoogleOAuthClientTypeWeb),
	}
}

func (s *Service) getOauth2ConfigForClientType(clientType model.GoogleOAuthClientType) OAuthConfig {
	clientType = model.NormalizeGoogleOAuthClientType(clientType)
	if clientType == model.GoogleOAuthClientTypeWeb && s.oauthConfig != nil {
		return s.oauthConfig
	}
	return getOauth2Config(clientType)
}

// WatchHandler is the gmail watch handler
func (s *Service) WatchHandler(ctx context.Context, payload GmailSyncRequest) (uint64, int64, error) {
	log := logger.Logger(ctx)
	oauthConfig := s.getOauth2ConfigForClientType(payload.OAuthClientType)
	tokenSource := oauthConfig.TokenSource(ctx, &oauth2.Token{
		RefreshToken: payload.RefreshToken,
	})
	token, err := tokenSource.Token()
	if err != nil {
		log.Error("Error while fetching token", "error", err)
		return 0, 0, errs.Wrap(errs.CodeInternalError, "Error while fetching token", err)
	}

	if payload.IsStop {
		log.Info("stopping gmail watch", "email", payload.Email)
		err := s.stopWatch(ctx, token, payload.Email, oauthConfig)
		if err != nil {
			return 0, 0, err
		}
		log.Info("stopped gmail watch", "email", payload.Email)
		return 0, 0, nil
	}

	historyID, expiration, err := s.setupWatch(ctx, payload.Email, token, oauthConfig)
	log.Info("gmail setup done", "historyId", historyID, "expiration", expiration, "err", err)

	return historyID, expiration, err
}

func (s *Service) stopWatch(ctx context.Context, token *oauth2.Token, email string, oauthConfig OAuthConfig) error {
	gmailAPI, err := s.gmailAPIFactory(ctx, token, oauthConfig)
	if err != nil {
		return err
	}
	return gmailAPI.StopWatch(ctx, email)
}

func (s *Service) setupWatch(
	ctx context.Context,
	email string,
	token *oauth2.Token,
	oauthConfig OAuthConfig,
) (uint64, int64, error) {
	gmailAPI, err := s.gmailAPIFactory(ctx, token, oauthConfig)
	if err != nil {
		return 0, 0, err
	}
	watchRequest := &gmail.WatchRequest{
		LabelIds:          []string{"INBOX"},
		LabelFilterAction: "include",
		TopicName:         fmt.Sprintf("projects/%s/topics/%s", s.config.ProjectID, s.config.PubsubTopic),
	}
	res, err := gmailAPI.SetupWatch(ctx, email, watchRequest)
	if err != nil {
		return 0, 0, err
	}
	logger.Logger(ctx).Info("gmail watch done", "res", res)
	return res.HistoryId, res.Expiration, nil
}

func (s *Service) GetMessageHistory(
	ctx context.Context,
	email string,
	historyId uint64,
	token *oauth2.Token,
	oauthConfig OAuthConfig,
) ([]EmailData, error) {
	log := logger.Logger(ctx)
	log.Info("GetMessageHistory", "email", email, "historyId", historyId)

	gmailAPI, err := s.gmailAPIFactory(ctx, token, oauthConfig)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "Error while creating gmail service", err)
	}
	historyRes, err := gmailAPI.ListHistory(ctx, email, historyId)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "Error while doing listCall.Do", err)
	}
	seen := make(map[string]bool)
	var msgData []EmailData
	for _, res := range historyRes.History {
		for _, addedMsg := range res.MessagesAdded {
			id := addedMsg.Message.Id
			if seen[id] {
				continue
			}
			seen[id] = true
			msgRes, err := gmailAPI.GetMessage(ctx, email, id)
			if err != nil {
				log.Error("error fetching message", "id", id, "error", err)
				return nil, nil
			}
			var bodyData strings.Builder
			for _, part := range msgRes.Payload.Parts {
				if part.MimeType == "text/html" {
					partData, err := base64.URLEncoding.DecodeString(part.Body.Data)
					if err != nil {
						log.Error("error decoding part", "error", err)
					}
					bodyData.Write(partData)
				}
			}
			headers := msgRes.Payload.Headers
			msgData = append(msgData, EmailData{MessageId: id, Headers: headers, Body: bodyData.String()})
		}
	}
	return msgData, nil
}

func (s *Service) IsTransactionEmail(emailHeader []*gmail.MessagePartHeader) bool {
	isTransaction := false
	for _, header := range emailHeader {
		if header.Name != "Subject" && header.Name != "From" {
			continue
		}
		valueLower := strings.ToLower(header.Value)
		slog.Debug("checking header value", "value", valueLower)
		if strings.Contains(valueLower, "txn") ||
			strings.Contains(valueLower, "alert : update") ||
			strings.Contains(valueLower, "alert :  update") ||
			strings.Contains(valueLower, "credit card") ||
			strings.Contains(valueLower, "debited") ||
			strings.Contains(valueLower, "instaalert") ||
			strings.Contains(valueLower, "view: account") {
			isTransaction = true
		}
	}

	return isTransaction
}
