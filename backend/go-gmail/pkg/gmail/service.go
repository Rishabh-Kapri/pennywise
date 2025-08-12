package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"gmail-transactions/pkg/config"

	"golang.org/x/oauth2"
	gmail "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type Service struct {
	config *config.Config
}

func NewService(config *config.Config) *Service {
	return &Service{config: config}
}

func (s *Service) SetupWatch(email string, token *oauth2.Token, oauthConfig *oauth2.Config) (uint64, error) {
	ctx := context.Background()
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
	log.Printf("GetMessageHistory: %v %v", email, historyId)
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
				log.Printf("Error while fetching message with id: %s %v", id, err.Error())
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
						log.Printf("Error while decoding part: %v", err.Error())
					}
					bodyData.Write(partData)
				}
			}
			headers := msgRes.Payload.Headers
			// parts := msgRes.Payload.Parts
			msgData = append(msgData, EmailData{Headers: headers, Body: bodyData.String()})
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
		log.Print(valueLower)
		if strings.Contains(valueLower, "txn") ||
			strings.Contains(valueLower, "alert : update") ||
			strings.Contains(valueLower, "alert :  update") ||
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
