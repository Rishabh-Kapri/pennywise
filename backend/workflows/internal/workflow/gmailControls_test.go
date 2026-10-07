package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestEmailWorkflowSkipsPausedMailbox(t *testing.T) {
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context, string) (*model.GoogleUserInfo, error) {
		return &model.GoogleUserInfo{GmailIngestionPaused: true}, nil
	}, activity.RegisterOptions{Name: "GetGoogleUserByEmail"})
	// No fetch, cursor update, or pipeline reporting activity is registered:
	// invoking any of them would fail the workflow.
	env.ExecuteWorkflow(EmailToTransactionWorkflow, model.EmailToTransactionWorflowInput{Email: "paused@example.com", HistoryId: 99})
	require.NoError(t, env.GetWorkflowError())
}

func TestEmailManualSyncUsesSavedCursorWithoutReplacingIt(t *testing.T) {
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	budgetID := uuid.New()
	env.RegisterActivityWithOptions(func(context.Context, string) (*model.GoogleUserInfo, error) {
		return &model.GoogleUserInfo{Email: "owner@example.com", GmailHistoryID: 123, BudgetID: budgetID}, nil
	}, activity.RegisterOptions{Name: "GetGoogleUserByEmail"})
	env.RegisterActivityWithOptions(func(ctx context.Context, input model.StartPipelineRunInput) (uuid.UUID, error) {
		require.Equal(t, model.PipelineTriggerManual, input.Trigger)
		return uuid.New(), nil
	}, activity.RegisterOptions{Name: "StartPipelineRun"})
	env.RegisterActivityWithOptions(func(ctx context.Context, input model.FetchAndParseEmailsInput) (model.EmailDataInput, error) {
		require.EqualValues(t, 123, input.HistoryID)
		require.Equal(t, budgetID, input.BudgetID)
		return model.EmailDataInput{BudgetID: budgetID}, nil
	}, activity.RegisterOptions{Name: "FetchEmailData"})
	env.RegisterActivityWithOptions(func(context.Context, model.ReportPipelineStatusInput) error { return nil }, activity.RegisterOptions{Name: "ReportPipelineStatus"})
	env.RegisterActivityWithOptions(func(ctx context.Context, input model.UpdateGmailHistoryInput) error {
		// Successful manual sync records its timestamp using the saved cursor,
		// rather than the input's zero-valued push history ID.
		require.EqualValues(t, 123, input.GmailHistoryID)
		return nil
	}, activity.RegisterOptions{Name: "UpdateGmailHistoryID"})
	env.ExecuteWorkflow(EmailToTransactionWorkflow, model.EmailToTransactionWorflowInput{Email: "owner@example.com", ManualSync: true})
	require.NoError(t, env.GetWorkflowError())
}

func TestEmailPushOnlyAdvancesCursorAfterSuccessfulFetch(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "success"
		if failed {
			name = "failed fetch"
		}
		t.Run(name, func(t *testing.T) {
			suite := testsuite.WorkflowTestSuite{}
			env := suite.NewTestWorkflowEnvironment()
			fetched, updated := false, false
			env.RegisterActivityWithOptions(func(context.Context, string) (*model.GoogleUserInfo, error) {
				return &model.GoogleUserInfo{GmailHistoryID: 123, BudgetID: uuid.New()}, nil
			}, activity.RegisterOptions{Name: "GetGoogleUserByEmail"})
			env.RegisterActivityWithOptions(func(context.Context, model.StartPipelineRunInput) (uuid.UUID, error) { return uuid.New(), nil }, activity.RegisterOptions{Name: "StartPipelineRun"})
			env.RegisterActivityWithOptions(func(context.Context, model.ReportPipelineStatusInput) error { return nil }, activity.RegisterOptions{Name: "ReportPipelineStatus"})
			env.RegisterActivityWithOptions(func(context.Context, model.FetchAndParseEmailsInput) (model.EmailDataInput, error) {
				if failed {
					return model.EmailDataInput{}, errors.New("Gmail unavailable")
				}
				fetched = true
				return model.EmailDataInput{}, nil
			}, activity.RegisterOptions{Name: "FetchEmailData"})
			env.RegisterActivityWithOptions(func(ctx context.Context, input model.UpdateGmailHistoryInput) error {
				require.True(t, fetched, "cursor must not advance before fetching succeeds")
				require.EqualValues(t, 999, input.GmailHistoryID)
				updated = true
				return nil
			}, activity.RegisterOptions{Name: "UpdateGmailHistoryID"})
			env.ExecuteWorkflow(EmailToTransactionWorkflow, model.EmailToTransactionWorflowInput{Email: "owner@example.com", HistoryId: 999})
			if failed {
				require.Error(t, env.GetWorkflowError())
				require.False(t, updated, "failed imports must remain available to Sync now")
			} else {
				require.NoError(t, env.GetWorkflowError())
				require.True(t, updated)
			}
		})
	}
}
