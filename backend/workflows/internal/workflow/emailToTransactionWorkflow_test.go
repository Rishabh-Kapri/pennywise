package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// registerActivityStubs registers no-op activities under the string names the
// workflows call, so env.OnActivity can mock them without the real services.
func registerActivityStubs(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(
		func(ctx context.Context, email string) (sharedModel.GoogleUserInfo, error) {
			return sharedModel.GoogleUserInfo{}, nil
		},
		activity.RegisterOptions{Name: "GetGoogleUserByEmail"},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, input sharedModel.UpdateGmailHistoryInput) error { return nil },
		activity.RegisterOptions{Name: "UpdateGmailHistoryID"},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, input sharedModel.FetchAndParseEmailsInput) (sharedModel.EmailDataInput, error) {
			return sharedModel.EmailDataInput{}, nil
		},
		activity.RegisterOptions{Name: "FetchEmailData"},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, input sharedModel.EmailDataInput) (sharedModel.ParsedEmailsInput, error) {
			return sharedModel.ParsedEmailsInput{}, nil
		},
		activity.RegisterOptions{Name: "ParseEmailData"},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, input sharedModel.ParsedEmailsInput) ([]sharedModel.CipherPredictionResult, error) {
			return nil, nil
		},
		activity.RegisterOptions{Name: "Predict"},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, input sharedModel.PredictionResultInput) ([]sharedModel.Transaction, error) {
			return nil, nil
		},
		activity.RegisterOptions{Name: "CreateTransactionAndCipherPrediction"},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, input sharedModel.StartPipelineRunInput) (uuid.UUID, error) {
			return uuid.Nil, nil
		},
		activity.RegisterOptions{Name: "StartPipelineRun"},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, input sharedModel.ReportPipelineStatusInput) error { return nil },
		activity.RegisterOptions{Name: "ReportPipelineStatus"},
	)
}

// pipelineReportRecorder captures every ReportPipelineStatus call for assertions.
type pipelineReportRecorder struct {
	reports []sharedModel.ReportPipelineStatusInput
}

func (r *pipelineReportRecorder) mock(env *testsuite.TestWorkflowEnvironment, runID uuid.UUID) {
	env.OnActivity("StartPipelineRun", mock.Anything, mock.Anything).Return(runID, nil)
	env.OnActivity("ReportPipelineStatus", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			input := args.Get(1).(sharedModel.ReportPipelineStatusInput)
			r.reports = append(r.reports, input)
		}).
		Return(nil)
}

func (r *pipelineReportRecorder) lastRunStatus() sharedModel.PipelineRunStatus {
	for i := len(r.reports) - 1; i >= 0; i-- {
		if r.reports[i].RunStatus != "" {
			return r.reports[i].RunStatus
		}
	}
	return ""
}

func (r *pipelineReportRecorder) eventStatuses(step sharedModel.PipelineStep) []sharedModel.PipelineEventStatus {
	var statuses []sharedModel.PipelineEventStatus
	for _, report := range r.reports {
		for _, event := range report.Events {
			if event.Step == step {
				statuses = append(statuses, event.Status)
			}
		}
	}
	return statuses
}

func newParsedWorkflowEnv(t *testing.T) (*testsuite.TestWorkflowEnvironment, *pipelineReportRecorder) {
	t.Helper()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(ParsedEmailToTransactionWorkflow, workflow.RegisterOptions{
		Name: sharedModel.ParsedEmailToTransactionWorkflowName,
	})
	registerActivityStubs(env)
	recorder := &pipelineReportRecorder{}
	recorder.mock(env, uuid.New())
	return env, recorder
}

func TestParsedEmailToTransactionWorkflow_HappyPath(t *testing.T) {
	env, recorder := newParsedWorkflowEnv(t)

	budgetID := uuid.New()
	input := sharedModel.EmailDataInput{
		BudgetID: budgetID,
		EmailData: []sharedModel.EmailData{
			{MessageId: "msg-1", Body: "txn email"},
			{MessageId: "msg-2", Body: "promo email"},
		},
	}

	parsed := sharedModel.ParsedEmailsInput{
		BudgetID: budgetID,
		ParsedEmails: []sharedModel.ParsedEmail{{
			MessageId:         "msg-1",
			EmailText:         "txn email",
			ExtractedMerchant: "Amazon",
			ExtractedAccount:  "XX1234",
			Amount:            -499,
			Date:              "2026-07-06",
			TransactionType:   "debit",
		}},
	}
	predictions := []sharedModel.CipherPredictionResult{{
		MessageId: "msg-1",
		AccountID: uuid.New(),
		Payee:     "Amazon",
		Category:  "Shopping",
		Source:    sharedModel.PredictionSourceVector,
	}}
	transactions := []sharedModel.Transaction{{ID: uuid.New()}}

	env.OnActivity("ParseEmailData", mock.Anything, mock.Anything).Return(parsed, nil)
	env.OnActivity("Predict", mock.Anything, mock.Anything).Return(predictions, nil)
	env.OnActivity("CreateTransactionAndCipherPrediction", mock.Anything, mock.Anything).
		Return(transactions, nil)

	env.ExecuteWorkflow(sharedModel.ParsedEmailToTransactionWorkflowName, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	require.Equal(t, sharedModel.PipelineRunStatusCompleted, recorder.lastRunStatus())
	require.Equal(t,
		[]sharedModel.PipelineEventStatus{
			sharedModel.PipelineEventStarted,
			sharedModel.PipelineEventSucceeded, // msg-1 extracted
			sharedModel.PipelineEventSkipped,   // msg-2 not a transaction
		},
		recorder.eventStatuses(sharedModel.PipelineStepParse),
	)
	require.Equal(t,
		[]sharedModel.PipelineEventStatus{
			sharedModel.PipelineEventStarted,
			sharedModel.PipelineEventSucceeded,
		},
		recorder.eventStatuses(sharedModel.PipelineStepPredict),
	)
	require.Equal(t,
		[]sharedModel.PipelineEventStatus{
			sharedModel.PipelineEventStarted,
			sharedModel.PipelineEventSucceeded,
		},
		recorder.eventStatuses(sharedModel.PipelineStepCreateTransactions),
	)
}

func TestParsedEmailToTransactionWorkflow_PredictParksThenRetrySignal(t *testing.T) {
	env, recorder := newParsedWorkflowEnv(t)

	budgetID := uuid.New()
	input := sharedModel.EmailDataInput{
		BudgetID:  budgetID,
		EmailData: []sharedModel.EmailData{{MessageId: "msg-1", Body: "txn email"}},
	}
	parsed := sharedModel.ParsedEmailsInput{
		BudgetID:     budgetID,
		ParsedEmails: []sharedModel.ParsedEmail{{MessageId: "msg-1", EmailText: "txn email"}},
	}
	predictions := []sharedModel.CipherPredictionResult{{MessageId: "msg-1", AccountID: uuid.New()}}

	env.OnActivity("ParseEmailData", mock.Anything, mock.Anything).Return(parsed, nil)
	// Fail every automatic attempt of the first Predict execution, succeed after
	// the manual retry signal.
	env.OnActivity("Predict", mock.Anything, mock.Anything).
		Return(nil, errors.New("ollama unavailable")).Times(3)
	env.OnActivity("Predict", mock.Anything, mock.Anything).Return(predictions, nil)
	env.OnActivity("CreateTransactionAndCipherPrediction", mock.Anything, mock.Anything).
		Return([]sharedModel.Transaction{{ID: uuid.New()}}, nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(sharedModel.RetryPredictSignal, nil)
	}, time.Hour)

	env.ExecuteWorkflow(sharedModel.ParsedEmailToTransactionWorkflowName, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	require.Equal(t, sharedModel.PipelineRunStatusCompleted, recorder.lastRunStatus())
	require.Equal(t,
		[]sharedModel.PipelineEventStatus{
			sharedModel.PipelineEventStarted,
			sharedModel.PipelineEventWaitingRetry,
			sharedModel.PipelineEventRetrySignaled,
			sharedModel.PipelineEventSucceeded,
		},
		recorder.eventStatuses(sharedModel.PipelineStepPredict),
	)
}

func TestParsedEmailToTransactionWorkflow_PredictFailsAfterWaitWindow(t *testing.T) {
	env, recorder := newParsedWorkflowEnv(t)

	budgetID := uuid.New()
	input := sharedModel.EmailDataInput{
		BudgetID:  budgetID,
		EmailData: []sharedModel.EmailData{{MessageId: "msg-1", Body: "txn email"}},
	}
	parsed := sharedModel.ParsedEmailsInput{
		BudgetID:     budgetID,
		ParsedEmails: []sharedModel.ParsedEmail{{MessageId: "msg-1", EmailText: "txn email"}},
	}

	env.OnActivity("ParseEmailData", mock.Anything, mock.Anything).Return(parsed, nil)
	env.OnActivity("Predict", mock.Anything, mock.Anything).
		Return(nil, errors.New("ollama unavailable"))

	env.ExecuteWorkflow(sharedModel.ParsedEmailToTransactionWorkflowName, input)

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())

	require.Equal(t, sharedModel.PipelineRunStatusFailed, recorder.lastRunStatus())
	statuses := recorder.eventStatuses(sharedModel.PipelineStepPredict)
	require.Equal(t, sharedModel.PipelineEventFailed, statuses[len(statuses)-1])
}
