package temporal

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/client"
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
	Cipher *client.CipherClient
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
	emailData, err := a.Gmail.GetMessageHistory(
		ctx,
		input.Email,
		input.HistoryID,
		token,
		gmail.WrapOAuthConfig(oauthConfig),
	)
	if err != nil {
		return result, err
	}

	var results []sharedModel.ParsedEmail
	log.Info("Fetched emails", "count", len(emailData))
	for _, data := range emailData {
		extracted, err := a.Cipher.ExtractTransactionFromEmail(
			ctx,
			client.EmailExtractionRequest{EmailHtml: data.Body},
		)
		if err != nil || extracted == nil {
			log.Error("error extracting email", "error", err)
			continue
		}

		if extracted.Amount == 0 || extracted.AccountCard == "" || extracted.Date == "" {
			log.Info("email not a transaction", "email", data.Body, "extracted", *extracted)
			continue
		}

		dateString := extracted.Date
		date, err := time.Parse("2006-01-02", dateString)
		if err != nil {
			log.Error("error parsing date", "error", err)
			continue
		}
		transactionType := "debit"
		if extracted.Amount > 0 {
			transactionType = "credit"
		}

		results = append(results, sharedModel.ParsedEmail{
			MessageId:       data.MessageId,
			EmailText:       extracted.EmailText,
			Amount:          extracted.Amount,
			Date:            date.Format("2006-01-02"),
			TransactionType: transactionType,
			Account:         "",
		})
	}
	result.ParsedEmails = results
	result.BudgetID = input.BudgetID

	return result, nil
}
