package temporal

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/gmail"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/parser"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"go.temporal.io/sdk/activity"
)

type GmailActivities struct {
	Auth   *auth.Service
	Gmail  *gmail.Service
	Parser *parser.EmailParser
}

func (a *GmailActivities) FetchAndParseEmails(
	ctx context.Context,
	input sharedModel.FetchAndParseEmailsInput,
) (result sharedModel.ParsedEmailsInput, err error) {
	ctx = utils.WithServiceName(ctx, "gmail-pubsub")
	activityInfo := activity.GetInfo(ctx)
	log := logger.Logger(ctx).With(
		"workflow_id", activityInfo.WorkflowExecution.ID,
		"workflow_run_id", activityInfo.WorkflowExecution.RunID,
		"activity_id", activityInfo.ActivityID,
		"activity_type", activityInfo.ActivityType.Name,
	)
	log.Info("FetchAndParseEmails", "email", input.Email, "historyId", input.HistoryID, "budgetId", input.BudgetID)

	// get access token from refresh token
	oauthConfig := a.Auth.GetOauth2ConfigForClientType(input.OAuthClientType)
	token, err := a.Auth.GetTokenFromRefresh(input.RefreshToken, input.OAuthClientType)
	if err != nil {
		return result, err
	}

	// fetch new emails from Gmail using history id
	emailData, err := a.Gmail.GetMessageHistory(ctx, input.Email, input.HistoryID, token, gmail.WrapOAuthConfig(oauthConfig))
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
			Date:            parsed.Date,
			TransactionType: parsed.TransactionType,
			Account:         "",
		})
	}
	result.ParsedEmails = results
	result.BudgetID = input.BudgetID

	return result, nil
}
