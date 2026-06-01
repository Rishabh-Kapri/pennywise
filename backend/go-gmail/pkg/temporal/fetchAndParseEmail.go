package temporal

import (
	"context"

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

func (a *GmailActivities) FetchEmailData(
	ctx context.Context,
	input sharedModel.FetchAndParseEmailsInput,
) (result sharedModel.EmailDataInput, err error) {
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

	result = sharedModel.EmailDataInput{
		EmailData: make([]sharedModel.EmailData, 0, len(emailData)),
		BudgetID:  input.BudgetID,
	}
	log.Info("Fetched emails", "count", len(emailData))

	for _, data := range emailData {
		log.Info("Email data", "messageId", data.MessageId, "body", data.Body, "headers", data.Headers)
		result.EmailData = append(result.EmailData, sharedModel.EmailData{
			MessageId: data.MessageId,
			Body:      data.Body,
		})
	}

	return result, nil
}
