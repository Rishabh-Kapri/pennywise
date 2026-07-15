package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

// pipelineFakes wires fake per-email activities into a test env and records
// what the create-transactions activity receives per invocation.
type pipelineFakes struct {
	mu sync.Mutex
	// predictFailing holds messageIds whose PredictEmail calls return a
	// retryable error; tests mutate it to simulate recovery.
	predictFailing map[string]bool
	// createBatches records the predictions passed to each
	// CreateTransactionAndCipherPrediction invocation.
	createBatches [][]sharedModel.CipherPredictionResult
}

func (f *pipelineFakes) failPredict(messageID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.predictFailing == nil {
		f.predictFailing = map[string]bool{}
	}
	f.predictFailing[messageID] = true
}

func (f *pipelineFakes) recoverPredict(messageID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.predictFailing, messageID)
}

func (f *pipelineFakes) register(t *testing.T, env *testsuite.TestWorkflowEnvironment) {
	t.Helper()

	env.RegisterWorkflowWithOptions(ParsedEmailToTransactionWorkflow, sdkworkflow.RegisterOptions{
		Name: sharedModel.ParsedEmailToTransactionWorkflowName,
	})

	env.RegisterActivityWithOptions(func(ctx context.Context, input sharedModel.ParseEmailInput) (sharedModel.ParseEmailResult, error) {
		if input.Email.Body == "skip" {
			return sharedModel.ParseEmailResult{Skipped: true, SkipReason: "not a transaction email"}, nil
		}
		return sharedModel.ParseEmailResult{
			Parsed: &sharedModel.ParsedEmail{
				MessageId: input.Email.MessageId,
				EmailText: input.Email.Body,
				Amount:    -100,
				Date:      "2026-07-14",
			},
		}, nil
	}, activity.RegisterOptions{Name: "ParseEmail"})

	env.RegisterActivityWithOptions(func(ctx context.Context, input sharedModel.PredictEmailInput) (sharedModel.PredictEmailResult, error) {
		f.mu.Lock()
		failing := f.predictFailing[input.Email.MessageId]
		f.mu.Unlock()
		if failing {
			return sharedModel.PredictEmailResult{}, errors.New("ollama unavailable")
		}
		return sharedModel.PredictEmailResult{
			Prediction: &sharedModel.CipherPredictionResult{
				MessageId:       input.Email.MessageId,
				OriginalRawText: input.Email.EmailText,
				Amount:          input.Email.Amount,
				Date:            input.Email.Date,
			},
		}, nil
	}, activity.RegisterOptions{Name: "PredictEmail"})

	env.RegisterActivityWithOptions(func(ctx context.Context, input sharedModel.PredictionResultInput) ([]sharedModel.Transaction, error) {
		f.mu.Lock()
		f.createBatches = append(f.createBatches, input.Predictions)
		f.mu.Unlock()
		txns := make([]sharedModel.Transaction, len(input.Predictions))
		return txns, nil
	}, activity.RegisterOptions{Name: "CreateTransactionAndCipherPrediction"})
}

func (f *pipelineFakes) batchMessageIDs(batch int) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0, len(f.createBatches[batch]))
	for _, p := range f.createBatches[batch] {
		ids = append(ids, p.MessageId)
	}
	return ids
}

func testInput(bodies map[string]string) sharedModel.EmailDataInput {
	input := sharedModel.EmailDataInput{BudgetID: uuid.New()}
	// deterministic order
	for _, id := range []string{"msg-1", "msg-2", "msg-3"} {
		if body, ok := bodies[id]; ok {
			input.EmailData = append(input.EmailData, sharedModel.EmailData{MessageId: id, Body: body})
		}
	}
	return input
}

// TestPerEmailPipelineHappyPath: skips are recorded, successes are committed,
// workflow completes without parking.
func TestPerEmailPipelineHappyPath(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	fakes := &pipelineFakes{}
	fakes.register(t, env)

	env.ExecuteWorkflow(sharedModel.ParsedEmailToTransactionWorkflowName, testInput(map[string]string{
		"msg-1": "skip",
		"msg-2": "txn email two",
		"msg-3": "txn email three",
	}))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Len(t, fakes.createBatches, 1)
	require.ElementsMatch(t, []string{"msg-2", "msg-3"}, fakes.batchMessageIDs(0))
}

// TestPerEmailPipelineIsolatesFailures: one email permanently failing predict
// must not block the other emails' transactions; the workflow eventually
// fails (after the retry window) but the successes were already committed.
func TestPerEmailPipelineIsolatesFailures(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	fakes := &pipelineFakes{}
	fakes.failPredict("msg-2")
	fakes.register(t, env)

	env.ExecuteWorkflow(sharedModel.ParsedEmailToTransactionWorkflowName, testInput(map[string]string{
		"msg-1": "skip",
		"msg-2": "txn email two",
		"msg-3": "txn email three",
	}))

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError(), "workflow must fail when an email stays unprocessed past the retry window")

	// msg-3's transaction was committed before the failure surfaced.
	require.Len(t, fakes.createBatches, 1)
	require.ElementsMatch(t, []string{"msg-3"}, fakes.batchMessageIDs(0))
}

// TestPerEmailPipelineRetrySignal: a parked workflow re-runs ONLY the failed
// email when the retry signal arrives, and completes once it succeeds.
func TestPerEmailPipelineRetrySignal(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	fakes := &pipelineFakes{}
	fakes.failPredict("msg-2")
	fakes.register(t, env)

	// After the workflow parks, simulate ollama recovering and an operator
	// hitting the retry endpoint (cipher signals by workflow ID).
	env.RegisterDelayedCallback(func() {
		fakes.recoverPredict("msg-2")
		env.SignalWorkflow(sharedModel.RetryPredictSignal, nil)
	}, time.Hour)

	env.ExecuteWorkflow(sharedModel.ParsedEmailToTransactionWorkflowName, testInput(map[string]string{
		"msg-2": "txn email two",
		"msg-3": "txn email three",
	}))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Round 1 committed msg-3; the post-signal round committed msg-2 only —
	// no duplicate create for msg-3.
	require.Len(t, fakes.createBatches, 2)
	require.ElementsMatch(t, []string{"msg-3"}, fakes.batchMessageIDs(0))
	require.ElementsMatch(t, []string{"msg-2"}, fakes.batchMessageIDs(1))
}
