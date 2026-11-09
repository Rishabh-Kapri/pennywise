package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/unimplemented"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

type enqueued struct {
	jobType string
	queue   string
	key     string
	payload []byte
}

type fakeBroker struct {
	job.Broker
	got []enqueued
	err error
}

func (b *fakeBroker) Enqueue(
	_ context.Context, jobType string, payload []byte, opts ...job.EnqueueOption,
) (job.EnqueueResult, error) {
	o, err := job.ResolveEnqueueOptions(jobType, payload, opts...)
	if err != nil {
		return job.EnqueueResult{}, err
	}
	if b.err != nil {
		return job.EnqueueResult{}, b.err
	}
	b.got = append(b.got, enqueued{jobType, o.Queue, o.IdempotencyKey, payload})
	return job.EnqueueResult{JobID: uuid.New()}, nil
}

type fakeChecker struct {
	verdict ports.TxnVerdict
	err     error
	budget  uuid.UUID
}

func (f *fakeChecker) Check(ctx context.Context, _ ports.Message) (ports.TxnVerdict, error) {
	f.budget, _ = utils.BudgetIDFromContext(ctx)
	return f.verdict, f.err
}

type fakePipeline struct {
	factory.MessagePipeline
	checker   ports.CheckTransaction
	extractor ports.Extract
	verifier  ports.Verifier
	predictor ports.Predictor
	err       error
}

func (f fakePipeline) Check(context.Context, uuid.UUID) (ports.CheckTransaction, error) {
	return f.checker, f.err
}

func (f fakePipeline) Extract(context.Context, uuid.UUID) (ports.Extract, error) {
	return f.extractor, f.err
}

func (f fakePipeline) Verify(context.Context, uuid.UUID) (ports.Verifier, error) {
	return f.verifier, f.err
}

func (f fakePipeline) Predict(context.Context, uuid.UUID) (ports.Predictor, error) {
	return f.predictor, f.err
}

type fakeVerifier struct {
	valid bool
	err   error
}

func (f fakeVerifier) Verify(context.Context, ports.Message, ports.ExtractedEntities) (ports.VerifyResult, error) {
	return ports.VerifyResult{Valid: f.valid}, f.err
}

type fakeExtractor struct{ err error }

func (f fakeExtractor) Extract(context.Context, ports.Message) (ports.ExtractedEntities, error) {
	return ports.ExtractedEntities{Merchant: "Cafe", Amount: 120}, f.err
}

type fakeMemory struct {
	found bool
	err   error
}

func (f fakeMemory) Lookup(context.Context, ports.Message, ports.ExtractedEntities) (ports.PredictionResult, bool, error) {
	return ports.PredictionResult{PayeeName: "Cafe", CategoryName: "Food"}, f.found, f.err
}

type fakePredictor struct {
	category string
	err      error
}

func (f fakePredictor) Predict(context.Context, ports.Message, ports.ExtractedEntities) (ports.PredictionResult, error) {
	return ports.PredictionResult{PayeeName: "Cafe", CategoryName: f.category}, f.err
}

var testMessage = model.ReceivedMessage{Source: "email", Provider: "gmail", ExternalID: "m1", Body: "spent 120"}

func newJob(t *testing.T, payload any) (*job.Job, uuid.UUID) {
	t.Helper()
	p, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	budget := uuid.New()
	return &job.Job{ID: uuid.New(), Type: "test", BudgetID: &budget, Payload: p}, budget
}

func TestRouting(t *testing.T) {
	entities := model.MessageEntitiesPayload{Message: testMessage}
	extract := model.MessageExtractPayload{Message: testMessage}

	tests := []struct {
		name      string
		handler   func(WorkerDeps) jobrunner.HandlerFunc
		deps      WorkerDeps
		payload   any
		wantType  string
		wantQueue string
		outcome   model.MessageOutcome
	}{
		{
			name:      "transaction goes to extract",
			handler:   messageReceived,
			deps:      WorkerDeps{Pipeline: fakePipeline{checker: &fakeChecker{verdict: ports.TxnVerdict{IsTransaction: true}}}},
			payload:   testMessage,
			wantType:  model.JobTypeMessageExtract,
			wantQueue: model.QueueCipher,
		},
		{
			name:      "not a transaction is reported",
			handler:   messageReceived,
			deps:      WorkerDeps{Pipeline: fakePipeline{checker: &fakeChecker{}}},
			payload:   testMessage,
			wantType:  model.JobTypePipelineReport,
			wantQueue: model.QueuePennywise,
			outcome:   model.MessageOutcomeNotTransaction,
		},
		{
			name:      "extract goes to verify",
			handler:   messageExtract,
			deps:      WorkerDeps{Pipeline: fakePipeline{extractor: fakeExtractor{}}},
			payload:   extract,
			wantType:  model.JobTypeMessageVerify,
			wantQueue: model.QueueCipher,
		},
		{
			name:      "valid goes to lookup",
			handler:   messageVerify,
			deps:      WorkerDeps{Pipeline: fakePipeline{verifier: fakeVerifier{valid: true}}},
			payload:   entities,
			wantType:  model.JobTypeMessageLookup,
			wantQueue: model.QueueCipher,
		},
		{
			name:      "invalid is reported",
			handler:   messageVerify,
			deps:      WorkerDeps{Pipeline: fakePipeline{verifier: fakeVerifier{}}},
			payload:   entities,
			wantType:  model.JobTypePipelineReport,
			wantQueue: model.QueuePennywise,
			outcome:   model.MessageOutcomeUnverified,
		},
		{
			name:      "memory hit creates the transaction",
			handler:   messageLookup,
			deps:      WorkerDeps{Memory: fakeMemory{found: true}},
			payload:   entities,
			wantType:  model.JobTypeTransactionCreate,
			wantQueue: model.QueuePennywise,
		},
		{
			name:      "memory miss goes to predict",
			handler:   messageLookup,
			deps:      WorkerDeps{Memory: fakeMemory{}},
			payload:   entities,
			wantType:  model.JobTypeMessagePredict,
			wantQueue: model.QueueCipher,
		},
		{
			name:      "prediction creates the transaction",
			handler:   messagePredict,
			deps:      WorkerDeps{Pipeline: fakePipeline{predictor: fakePredictor{category: "Food"}}},
			payload:   entities,
			wantType:  model.JobTypeTransactionCreate,
			wantQueue: model.QueuePennywise,
		},
		{
			name:      "no prediction is reported",
			handler:   messagePredict,
			deps:      WorkerDeps{Pipeline: fakePipeline{predictor: fakePredictor{}}},
			payload:   entities,
			wantType:  model.JobTypePipelineReport,
			wantQueue: model.QueuePennywise,
			outcome:   model.MessageOutcomeNoPrediction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &fakeBroker{}
			tt.deps.Broker = b
			j, budget := newJob(t, tt.payload)

			raw, err := tt.handler(tt.deps)(context.Background(), j)
			if err != nil {
				t.Fatal(err)
			}
			if len(b.got) != 1 {
				t.Fatalf("queued %d jobs", len(b.got))
			}
			got := b.got[0]
			if got.jobType != tt.wantType || got.queue != tt.wantQueue {
				t.Fatalf("queued %s on %s", got.jobType, got.queue)
			}
			if want := testMessage.IdempotencyKey(budget); got.key != want {
				t.Fatalf("key %q, want %q", got.key, want)
			}

			var res model.MessageStepResult
			if err := json.Unmarshal(raw, &res); err != nil {
				t.Fatal(err)
			}
			if res.NextJobType != tt.wantType || res.NextJobID == nil || res.Outcome != tt.outcome {
				t.Fatalf("result %+v", res)
			}
		})
	}
}

func TestPredictionSource(t *testing.T) {
	for name, tc := range map[string]struct {
		handler func(WorkerDeps) jobrunner.HandlerFunc
		deps    WorkerDeps
		want    model.MessagePredictionSource
	}{
		"memory":    {messageLookup, WorkerDeps{Memory: fakeMemory{found: true}}, model.MessagePredictionMemory},
		"predictor": {messagePredict, WorkerDeps{Pipeline: fakePipeline{predictor: fakePredictor{category: "Food"}}}, model.MessagePredictionPredictor},
	} {
		t.Run(name, func(t *testing.T) {
			b := &fakeBroker{}
			tc.deps.Broker = b
			j, _ := newJob(t, model.MessageEntitiesPayload{Message: testMessage})
			if _, err := tc.handler(tc.deps)(context.Background(), j); err != nil {
				t.Fatal(err)
			}
			var p model.TransactionCreatePayload
			if err := json.Unmarshal(b.got[0].payload, &p); err != nil {
				t.Fatal(err)
			}
			if p.Prediction.Source != tc.want || p.Prediction.CategoryName != "Food" {
				t.Fatalf("%+v", p.Prediction)
			}
		})
	}
}

func TestPortFailures(t *testing.T) {
	boom := errors.New("boom")
	entities := model.MessageEntitiesPayload{Message: testMessage}

	tests := []struct {
		name    string
		handler jobrunner.HandlerFunc
		payload any
		code    errs.Code
	}{
		{"check", messageReceived(WorkerDeps{Pipeline: fakePipeline{checker: &fakeChecker{err: boom}}}), testMessage, errs.CodeMessageCheckFailed},
		{"extract", messageExtract(WorkerDeps{Pipeline: fakePipeline{extractor: fakeExtractor{err: boom}}}), model.MessageExtractPayload{}, errs.CodeMessageExtractFailed},
		{"verify", messageVerify(WorkerDeps{Pipeline: fakePipeline{verifier: fakeVerifier{err: boom}}}), entities, errs.CodeMessageVerifyFailed},
		{"lookup", messageLookup(WorkerDeps{Memory: fakeMemory{err: boom}}), entities, errs.CodeMessageLookupFailed},
		{"predict", messagePredict(WorkerDeps{Pipeline: fakePipeline{predictor: fakePredictor{err: boom}}}), entities, errs.CodeMessagePredictFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j, _ := newJob(t, tt.payload)
			if _, err := tt.handler(context.Background(), j); !errs.HasCode(err, tt.code) || !errors.Is(err, boom) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestStandInsFailVisibly(t *testing.T) {
	j, _ := newJob(t, model.MessageExtractPayload{Message: testMessage})
	deps := WorkerDeps{Broker: &fakeBroker{}, Pipeline: fakePipeline{extractor: unimplemented.NewExtract()}}
	_, err := messageExtract(deps)(context.Background(), j)
	if !errs.HasCode(err, errs.CodeNotImplemented) {
		t.Fatalf("got %v", err)
	}
}

func TestBadJobs(t *testing.T) {
	deps := WorkerDeps{Broker: &fakeBroker{}, Pipeline: fakePipeline{checker: &fakeChecker{}}}

	j, _ := newJob(t, testMessage)
	j.BudgetID = nil
	if _, err := messageReceived(deps)(context.Background(), j); !errs.HasCode(err, errs.CodeJobInvalidPayload) {
		t.Fatalf("no budget: %v", err)
	}

	j, _ = newJob(t, testMessage)
	j.Payload = []byte("nope")
	if _, err := messageReceived(deps)(context.Background(), j); !errs.HasCode(err, errs.CodeJobInvalidPayload) {
		t.Fatalf("bad payload: %v", err)
	}
}

func TestEnqueueFailureIsRetried(t *testing.T) {
	checker := &fakeChecker{verdict: ports.TxnVerdict{IsTransaction: true}}
	deps := WorkerDeps{Broker: &fakeBroker{err: errors.New("down")}, Pipeline: fakePipeline{checker: checker}}
	j, budget := newJob(t, testMessage)

	_, err := messageReceived(deps)(context.Background(), j)
	if !errs.HasCode(err, errs.CodeJobEnqueueFailed) {
		t.Fatalf("got %v", err)
	}
	if checker.budget != budget {
		t.Fatalf("port saw budget %s, want %s", checker.budget, budget)
	}
}

func TestToPortMessageMapsAPISource(t *testing.T) {
	if got := toPortMessage(model.ReceivedMessage{Source: "api"}).Source; got != ports.TextSource {
		t.Fatalf("got %s", got)
	}
}

func TestToPortMessageTurnsEmailHTMLIntoText(t *testing.T) {
	got := toPortMessage(model.ReceivedMessage{Source: "email", Body: "<html><body><p>Rs. 450 debited</p></body></html>"})
	if got.Body != "Rs. 450 debited" {
		t.Fatalf("email body = %q", got.Body)
	}
	sms := toPortMessage(model.ReceivedMessage{Source: "sms", Body: "a <b> c"})
	if sms.Body != "a <b> c" {
		t.Fatalf("sms body = %q", sms.Body)
	}
}

func TestStepStoresItsOutput(t *testing.T) {
	broker := &fakeBroker{}
	deps := WorkerDeps{Broker: broker, Pipeline: fakePipeline{verifier: fakeVerifier{valid: true}}}
	j, _ := newJob(t, model.MessageEntitiesPayload{Message: testMessage})

	raw, err := messageVerify(deps)(context.Background(), j)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Output      ports.VerifyResult `json:"output"`
		NextJobType string             `json:"nextJobType"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Output.Valid || got.NextJobType != model.JobTypeMessageLookup {
		t.Fatalf("result = %s", raw)
	}
}
