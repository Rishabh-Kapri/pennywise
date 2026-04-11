package temporal

import (
	"context"
	"gmail-transactions/pkg/auth"
	"gmail-transactions/pkg/gmail"
	"gmail-transactions/pkg/parser"
	"gmail-transactions/pkg/pennywise-api"

	"github.com/Rishabh-Kapri/pennywise/backend/workflows"
)

type GmailActivities struct {
	Auth      *auth.Service
	Gmail     *gmail.Service
	Parser    *parser.EmailParser
	Pennywise *pennywise.Service
}

func (a *GmailActivities) FetchAndParseEmails(ctx context.Context, input workflows.EmailWorflowInput) ([]workflows.ParsedEmail, error) {
	// fetch refresh token
	refreshToken, err := a.Pennywise.GetUserRefreshToken(ctx, input.Email)
	if err != nil {
		return nil, err
	}

	// get access token from refresh token
	oauthConfig := a.Auth.GetOauth2Config()
	token, err := a.Auth.GetTokenFromRefresh(refreshToken)
	if err != nil {
		return nil, err
	}

	// get saved history id for user
	prevHistoryId, err := a.Pennywise.GetUserHistoryId(ctx, input.Email)
	if err != nil {
		return nil, err
	}

	// update history id first
	if err := a.Pennywise.UpdateUserHistoryId(ctx, input.Email, input.HistoryId); err != nil {
		return nil, err
	}

	// fetch new emails from Gmail using history id
	emailData, err := a.Gmail.GetMessageHistory(input.Email, prevHistoryId, token, oauthConfig)
	if err != nil {
		return nil, err
	}

	var results []workflows.ParsedEmail
	for _, data := range emailData {
		isTransaction, defaultAccount := a.Gmail.IsTransactionEmail(data.Headers)
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
		results = append(results, workflows.ParsedEmail{
			MessageId:       data.MessageId,
			EmailText:       parsed.Text,
			Amount:          parsed.Amount,
			TransactionType: parsed.TransactionType,
			DefaultAccount:  defaultAccount,
			Payee:           parsed.Payee,
		})
	}

	return results, nil
}
