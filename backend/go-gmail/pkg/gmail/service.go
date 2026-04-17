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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmail "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type Service struct {
	config *config.Config
}

func getOauth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		// RedirectURL:  os.Getenv("CALLBACK_URL"),
		RedirectURL: "postmessage",
		Endpoint:    google.Endpoint,
		Scopes:      []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
	}
}

func NewService() *Service {
	config := config.LoadConfig()
	return &Service{config: config}
}

// WatchHandler is the gmail watch handler
func (s *Service) WatchHandler(ctx context.Context, payload GmailSyncRequest) (uint64, error) {
	config := getOauth2Config()
	logger := logger.Logger(ctx)
	tokenSource := config.TokenSource(ctx, &oauth2.Token{
		RefreshToken: payload.RefreshToken,
	})
	token, err := tokenSource.Token()
	logger.Info("token", "token", token)
	if err != nil {
		logger.Error("Error while fetching token", "error", err)
		return 0, errs.Wrap(errs.CodeInternalError, "Error while fetching token", err)
	}

	if payload.IsStop {
		logger.Info("stopping gmail watch", "email", payload.Email)
		err := s.stopWatch(ctx, token, config, payload.Email)
		if err != nil {
			return 0, err
		}
		logger.Info("stopped gmail watch", "email", payload.Email)
		return 0, nil
	}

	// historyID := uint64(1)
	historyID, err := s.setupWatch(ctx, payload.Email, token, config)
	logger.Info("gmail setup done", "historyId", historyID, "err", err)

	return historyID, nil
}


func (s *Service) stopWatch(ctx context.Context, token *oauth2.Token, oauthConfig *oauth2.Config, email string) error {
	gmailService, err := gmail.NewService(ctx, option.WithTokenSource(oauthConfig.TokenSource(ctx, token)))
	if err != nil {
		return err
	}
	gmailService.Users.Stop(email)
	return nil
}

func (s *Service) setupWatch(ctx context.Context, email string, token *oauth2.Token, oauthConfig *oauth2.Config) (uint64, error) {
	gmailService, err := gmail.NewService(ctx, option.WithTokenSource(oauthConfig.TokenSource(ctx, token)))
	if err != nil {
		return 0, err
	}
	watchRequest := &gmail.WatchRequest{
		LabelIds:          []string{"INBOX"},
		LabelFilterAction: "include",
		TopicName:         fmt.Sprintf("projects/%s/topics/%s", s.config.ProjectID, s.config.PubsubTopic),
	}
	gmailUserService := gmail.NewUsersService(gmailService)
	res, err := gmailUserService.Watch(email, watchRequest).Do()
	if err != nil {
		return 0, err
	}
	return res.HistoryId, nil
}

func (s *Service) GetMessageHistory(email string, historyId uint64, token *oauth2.Token, oauthConfig *oauth2.Config) ([]EmailData, error) {
	slog.Info("GetMessageHistory", "email", email, "historyId", historyId)
	ctx := context.Background()
	gmailService, err := gmail.NewService(ctx, option.WithTokenSource(oauthConfig.TokenSource(ctx, token)))
	if err != nil {
		return nil, fmt.Errorf("Error while creating gmail service: %v", err.Error())
	}
	listCall := gmailService.Users.History.List(email)
	listCall.StartHistoryId(historyId)
	historyRes, err := listCall.Do()
	if err != nil {
		return nil, fmt.Errorf("Error while doing listCall.Do: %v", err.Error())
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
			msgRes, err := gmailService.Users.Messages.Get(email, id).Do()
			if err != nil {
				slog.Error("error fetching message", "id", id, "error", err)
				return nil, err
			}
			// body, err := base64.URLEncoding.DecodeString(msgRes.Payload.Body.Data)
			// if err != nil {
			// 	log.Printf("Error while decoding body: %v", err.Error())
			// }
			var bodyData strings.Builder
			for _, part := range msgRes.Payload.Parts {
				if part.MimeType == "text/html" {
					partData, err := base64.URLEncoding.DecodeString(part.Body.Data)
					if err != nil {
						slog.Error("error decoding part", "error", err)
					}
					bodyData.Write(partData)
				}
			}
			headers := msgRes.Payload.Headers
			// parts := msgRes.Payload.Parts
			msgData = append(msgData, EmailData{MessageId: id, Headers: headers, Body: bodyData.String()})
		}
	}
	return msgData, nil
}

func (s *Service) IsTransactionEmail(emailHeader []*gmail.MessagePartHeader) (bool, string) {
	isTransaction := false
	accName := ""
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
		if strings.Contains(valueLower, "credit card") {
			accName = "HDFC Credit Card"
		}
	}
	if isTransaction && accName != "HDFC Credit Card" {
		accName = "HDFC (Salary)"
	}

	return isTransaction, accName
}
