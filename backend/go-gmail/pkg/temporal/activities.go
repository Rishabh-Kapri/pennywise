package temporal

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/client"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/gmail"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/parser"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type GmailActivities struct {
	Auth      *auth.Service
	Gmail     *gmail.Service
	Parser    *parser.EmailParser
	Pennywise *client.PennywiseClient
}

func (a *GmailActivities) FetchAndParseEmails(
	ctx context.Context,
	input sharedModel.EmailWorflowInput,
) (result sharedModel.ParsedEmailsInput, err error) {
	log := logger.Logger(ctx)
	log.Info("FetchAndParseEmails", "input", input)
	// fetch user info (including refresh token and history id) by email
	userInfo, err := a.Pennywise.GetUser(ctx, input.Email)
	if err != nil {
		return result, err
	}

	// get access token from refresh token
	oauthConfig := a.Auth.GetOauth2Config()
	token, err := a.Auth.GetTokenFromRefresh(userInfo.RefreshToken)
	if err != nil {
		return result, err
	}

	prevHistoryId := uint64(userInfo.GmailHistoryID)

	// update history id first
	if err := a.Pennywise.UpdateUserHistoryId(ctx, input.Email, input.HistoryId); err != nil {
		return result, err
	}

	// fetch new emails from Gmail using history id
	emailData, err := a.Gmail.GetMessageHistory(ctx, input.Email, prevHistoryId, token, oauthConfig)
	if err != nil {
		return result, err
	}

	var results []sharedModel.ParsedEmail
	log.Info("Fetched emails", "count", len(emailData))
	for _, data := range emailData {
		isTransaction := a.Gmail.IsTransactionEmail(data.Headers)
		if !isTransaction {
			continue
		}
		parsed, err := a.Parser.ParseEmail(data.Body)
		if err != nil {
			continue
		}
		if parsed.Amount == 0 {
			continue
		}
		results = append(results, sharedModel.ParsedEmail{
			MessageId:       data.MessageId,
			EmailText:       parsed.Text,
			Amount:          parsed.Amount,
			TransactionType: parsed.TransactionType,
			Account:         "",
		})
	}
	result.ParsedEmails = results
	result.BudgetID = userInfo.BudgetID

	return result, nil
}
