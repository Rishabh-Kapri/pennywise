package runner

import (
	"context"
	"fmt"
	"sync"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/gmail"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/parser"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/pennywise-api"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/prediction"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
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
	// @TODO: Evict based on LRU, not nuking the whole cache
	if len(s.processedMsgIds) >= maxProcessedMsgIds {
		s.processedMsgIds = make(map[string]bool)
	}
	s.processedMsgIds[messageId] = true
	return false
}

func (s *Runner) ProcessGmailHistoryId(ctx context.Context, eventData EventData) error {
	log := logger.Logger(ctx)
	log.Info("processing event", "eventData", eventData)

	// Fetch user info (including budgetId and refresh token) by email — no budget scoping needed
	userInfo, err := s.pennywise.GetUser(ctx, eventData.Email)
	if err != nil {
		return fmt.Errorf("Failed to get user info: %w", err)
	}
	log.Info("fetched user info", "budgetId", userInfo.BudgetID, "historyId", userInfo.GmailHistoryID)

	// Set budget ID in context so all subsequent transport calls auto-inject X-Budget-ID
	budgetUUID, err := uuid.Parse(userInfo.BudgetID)
	if err != nil {
		return fmt.Errorf("Failed to parse budget ID: %w", err)
	}
	ctx = utils.WithBudgetID(ctx, budgetUUID)

	oauthconfig := s.auth.GetOauth2Config()
	token, err := s.auth.GetTokenFromRefresh(userInfo.RefreshToken)
	if err != nil {
		return fmt.Errorf("Failed to get access token: %w", err)
	}

	prevHistoryId := uint64(userInfo.GmailHistoryID)
	log.Info("received history id", "prevHistoryId", prevHistoryId)

	if err := s.pennywise.UpdateUserHistoryId(ctx, eventData.Email, eventData.HistoryId); err != nil {
		return fmt.Errorf("Failed to update history id: %w", err)
	}

	emailData, err := s.gmail.GetMessageHistory(eventData.Email, prevHistoryId, token, oauthconfig)
	if err != nil {
		return fmt.Errorf("Failed to get message history: %w", err)
	}

	log.Info("email data fetched", "count", len(emailData))
	for _, data := range emailData {
		// Skip already processed Gmail messages (cross-call dedup)
		if s.isProcessed(data.MessageId) {
			log.Info("duplicate message ID detected, skipping", "messageId", data.MessageId)
			continue
		}

		isTransaction, defaultAccount := s.gmail.IsTransactionEmail(data.Headers)
		if !isTransaction {
			log.Info("not a transaction, skipping")
			continue
		}
		parsedDetails, err := s.parser.ParseEmail(data.Body)
		if err != nil {
			log.Error("failed to parse email", "error", err)
			continue
		}
		log.Info("parsed email details", "details", parsedDetails)
		if parsedDetails.Amount == 0 {
			log.Info("amount is 0, skipping")
			continue
		}
		// @TODO: call the cipher service here
		predictedFields, err := s.prediction.GetPredictedFields(ctx, parsedDetails, defaultAccount)
		if err != nil {
			log.Error("error getting predicted fields", "error", err)
			return err
		}
		createdTxn, err := s.pennywise.CreateTransaction(ctx, parsedDetails, predictedFields)
		if err != nil {
			log.Error("error creating transaction", "error", err)
			return err
		}
		err = s.pennywise.CreatePrediction(ctx, parsedDetails, predictedFields, createdTxn)
		if err != nil {
			log.Error("error creating prediction", "error", err)
			return err
		}
	}
	return nil
}
