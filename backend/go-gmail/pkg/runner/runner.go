package runner

import (
	"fmt"
	"log"

	"gmail-transactions/pkg/auth"
	"gmail-transactions/pkg/gmail"
	"gmail-transactions/pkg/parser"
	"gmail-transactions/pkg/pennywise-api"
	"gmail-transactions/pkg/prediction"
	"gmail-transactions/pkg/storage"
)

type Runner struct {
	auth        *auth.Service
	gmail       *gmail.Service
	parser      *parser.EmailParser
	prediction  *prediction.Service
	storage     *storage.Service
	pennywiseService *pennywise.Service
}

type EventData struct {
	Email     string `json:"emailAddress"`
	HistoryId uint64 `json:"historyId"`
}

func NewRunner(
	authService *auth.Service,
	gmailService *gmail.Service,
	parserService *parser.EmailParser,
	predictionService *prediction.Service,
	storageService *storage.Service,
	pennywiseService *pennywise.Service,
) *Runner {
	return &Runner{
		auth:       authService,
		gmail:      gmailService,
		parser:     parserService,
		prediction: predictionService,
		storage:    storageService,
	}
}

func (s *Runner) ProcessGmailHistoryId(eventData EventData) error {
	log.Println("Processing event", eventData)

	refreshToken, err := s.storage.GetRefreshToken(eventData.Email)
	if err != nil {
		return fmt.Errorf("Failed to get refresh token: %w", err)
	}

	oauthconfig := s.auth.GetOauth2Config()
	token, err := s.auth.GetTokenFromRefresh(refreshToken)
	if err != nil {
		return fmt.Errorf("Failed to get access token: %w", err)
	}

	prevHistoryId, err := s.storage.GetPrevHistoryId(eventData.Email)
	if err != nil {
		return fmt.Errorf("Failed to get prev history id: %w", err)
	}

	if err := s.storage.UpdateHistoryId(eventData.Email, eventData.HistoryId); err != nil {
		return fmt.Errorf("Failed to update history id: %w", err)
	}

	emailData, err := s.gmail.GetMessageHistory(eventData.Email, prevHistoryId, token, oauthconfig)
	if err != nil {
		return fmt.Errorf("Failed to get message history: %w", err)
	}

	log.Print("Email Data fetched:", len(emailData))
	for _, data := range emailData {
		isTransaction, defaultAccount := s.gmail.IsTransactionEmail(data.Headers)
		if !isTransaction {
			log.Printf("Not a transaction, skipping!\n")
			continue
		}
		parsedDetails, err := s.parser.ParseEmail(data.Body)
		if err != nil {
			log.Printf("Failed to parse email: %v", err)
			return err
		}
		log.Printf("Parsed email details: %v", parsedDetails)
		if parsedDetails.Amount == 0 {
			log.Printf("Amount is 0, skipping!\n")
			continue
		}
		predictedFields, err := s.prediction.GetPredictedFields(parsedDetails, defaultAccount)
		if err != nil {
			log.Printf("Error while getting predicted fields: %v", err)
			return err
		}
		createdTxn, err := s.pennywiseService.CreateTransaction(parsedDetails, predictedFields)
		if err != nil {
			log.Printf("Error while creating transaction: %v", err)
			return err
		}
		err = s.pennywiseService.CreatePrediction(parsedDetails, predictedFields, createdTxn)
		if err!= nil {
			log.Printf("Error while creating prediction: %v", err)
			return err
		}
	}
	return nil
}
