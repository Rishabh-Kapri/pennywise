package runner

import (
	"fmt"
	"log/slog"
	"sync"

	"gmail-transactions/pkg/auth"
	"gmail-transactions/pkg/gmail"
	"gmail-transactions/pkg/parser"
	"gmail-transactions/pkg/pennywise-api"
	"gmail-transactions/pkg/prediction"
)

const maxProcessedMsgIds = 1000

type Runner struct {
	auth            *auth.Service
	gmail           *gmail.Service
	parser          *parser.EmailParser
	prediction      *prediction.Service
	pennywise       *pennywise.Service
	mu              sync.Mutex
	processedMsgIds map[string]bool
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
	pennywiseService *pennywise.Service,
) *Runner {
	return &Runner{
		auth:            authService,
		gmail:           gmailService,
		parser:          parserService,
		prediction:      predictionService,
		pennywise:       pennywiseService,
		processedMsgIds: make(map[string]bool),
	}
}


// isProcessed checks if a Gmail message ID has already been processed.
// Returns true if already seen, false otherwise (and marks it as seen).
func (s *Runner) isProcessed(messageId string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.processedMsgIds[messageId] {
		return true
	}

	// Evict old entries if cache is too large
	if len(s.processedMsgIds) >= maxProcessedMsgIds {
		s.processedMsgIds = make(map[string]bool)
	}
	s.processedMsgIds[messageId] = true
	return false
}

func (s *Runner) ProcessGmailHistoryId(eventData EventData) error {
	slog.Info("processing event", "eventData", eventData)

	refreshToken, err := s.pennywise.GetUserRefreshToken(eventData.Email)
	if err != nil {
		return fmt.Errorf("Failed to get refresh token: %w", err)
	}

	oauthconfig := s.auth.GetOauth2Config()
	token, err := s.auth.GetTokenFromRefresh(refreshToken)
	if err != nil {
		return fmt.Errorf("Failed to get access token: %w", err)
	}

	prevHistoryId, err := s.pennywise.GetUserHistoryId(eventData.Email)
	if err != nil {
		return fmt.Errorf("Failed to get prev history id: %w", err)
	}
	slog.Info("received history id", "prevHistoryId", prevHistoryId)

	if err := s.pennywise.UpdateUserHistoryId(eventData.Email, eventData.HistoryId); err != nil {
		return fmt.Errorf("Failed to update history id: %w", err)
	}

	emailData, err := s.gmail.GetMessageHistory(eventData.Email, prevHistoryId, token, oauthconfig)
	if err != nil {
		return fmt.Errorf("Failed to get message history: %w", err)
	}

	slog.Info("email data fetched", "count", len(emailData))
	for _, data := range emailData {
		// Skip already processed Gmail messages (cross-call dedup)
		if s.isProcessed(data.MessageId) {
			slog.Info("duplicate message ID detected, skipping", "messageId", data.MessageId)
			continue
		}

		isTransaction, defaultAccount := s.gmail.IsTransactionEmail(data.Headers)
		if !isTransaction {
			slog.Info("not a transaction, skipping")
			continue
		}
		parsedDetails, err := s.parser.ParseEmail(data.Body)
		if err != nil {
			slog.Error("failed to parse email", "error", err)
			continue
		}
		slog.Info("parsed email details", "details", parsedDetails)
		if parsedDetails.Amount == 0 {
			slog.Info("amount is 0, skipping")
			continue
		}
		predictedFields, err := s.prediction.GetPredictedFields(parsedDetails, defaultAccount)
		if err != nil {
			slog.Error("error getting predicted fields", "error", err)
			return err
		}
		createdTxn, err := s.pennywise.CreateTransaction(parsedDetails, predictedFields)
		if err != nil {
			slog.Error("error creating transaction", "error", err)
			return err
		}
		err = s.pennywise.CreatePrediction(parsedDetails, predictedFields, createdTxn)
		if err != nil {
			slog.Error("error creating prediction", "error", err)
			return err
		}
	}
	return nil
}
