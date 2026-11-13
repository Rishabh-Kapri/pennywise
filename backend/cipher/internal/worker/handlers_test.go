package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

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
	memory    ports.MemoryLookup
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

func (f fakePipeline) Lookup(context.Context, uuid.UUID) (ports.MemoryLookup, error) {
	return f.memory, f.err
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
			outcome:   model.MessageOutcomeNeedsReview,
		},
		{
			name:      "memory hit creates the transaction",
			handler:   messageLookup,
			deps:      WorkerDeps{Pipeline: fakePipeline{memory: fakeMemory{found: true}}},
			payload:   entities,
			wantType:  model.JobTypeTransactionCreate,
			wantQueue: model.QueuePennywise,
		},
		{
			name:      "memory miss goes to predict",
			handler:   messageLookup,
			deps:      WorkerDeps{Pipeline: fakePipeline{memory: fakeMemory{}}},
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
		"memory":    {messageLookup, WorkerDeps{Pipeline: fakePipeline{memory: fakeMemory{found: true}}}, model.MessagePredictionMemory},
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
		{"lookup", messageLookup(WorkerDeps{Pipeline: fakePipeline{memory: fakeMemory{err: boom}}}), entities, errs.CodeMessageLookupFailed},
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

func decodeReport(t *testing.T, payload json.RawMessage) model.PipelineReportPayload {
	t.Helper()
	var report model.PipelineReportPayload
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func TestCheckParksLowConfidence(t *testing.T) {
	low := 0.55
	for name, isTransaction := range map[string]bool{"unsure yes": true, "unsure no": false} {
		t.Run(name, func(t *testing.T) {
			b := &fakeBroker{}
			checker := &fakeChecker{verdict: ports.TxnVerdict{IsTransaction: isTransaction, Type: "debit", Confidence: &low}}
			deps := WorkerDeps{Broker: b, ParkLowConfidence: true, Pipeline: fakePipeline{checker: checker}}
			j, budget := newJob(t, testMessage)

			if _, err := messageReceived(deps)(context.Background(), j); err != nil {
				t.Fatal(err)
			}
			if len(b.got) != 1 || b.got[0].jobType != model.JobTypePipelineReport || b.got[0].queue != model.QueuePennywise {
				t.Fatalf("queued %+v", b.got)
			}
			report := decodeReport(t, b.got[0].payload)
			if report.Outcome != model.MessageOutcomeNeedsReview || report.Review == nil || report.Review.Step != "check" {
				t.Fatalf("report %+v", report)
			}

			resume := report.Review.Resume
			if resume == nil || resume.JobType != model.JobTypeMessageExtract || resume.Queue != model.QueueCipher {
				t.Fatalf("resume %+v", resume)
			}
			if resume.IdempotencyKey == testMessage.IdempotencyKey(budget) {
				t.Fatal("the resumed run shares keys with the parked one, so its report would be dropped")
			}
			var payload model.MessageExtractPayload
			if err := json.Unmarshal(resume.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if !payload.Verdict.IsTransaction || payload.Message.IdempotencyKey(budget) != resume.IdempotencyKey {
				t.Fatalf("resume payload %+v", payload)
			}
		})
	}
}

func TestCheckConfidenceGate(t *testing.T) {
	low, high := 0.55, 0.95
	tests := []struct {
		name          string
		park          bool
		isTransaction bool
		confidence    *float64
		want          string
	}{
		{"logged only until parking is on", false, true, &low, model.JobTypeMessageExtract},
		{"confident yes", true, true, &high, model.JobTypeMessageExtract},
		{"confident no", true, false, &high, model.JobTypePipelineReport},
		{"a no without a score ends the message", true, false, nil, model.JobTypePipelineReport},
		{"a yes without a score goes on", true, true, nil, model.JobTypeMessageExtract},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &fakeBroker{}
			checker := &fakeChecker{verdict: ports.TxnVerdict{IsTransaction: tt.isTransaction, Confidence: tt.confidence}}
			deps := WorkerDeps{Broker: b, ParkLowConfidence: tt.park, Pipeline: fakePipeline{checker: checker}}
			j, _ := newJob(t, testMessage)
			if _, err := messageReceived(deps)(context.Background(), j); err != nil {
				t.Fatal(err)
			}
			if b.got[0].jobType != tt.want {
				t.Fatalf("queued %s, want %s", b.got[0].jobType, tt.want)
			}
			if b.got[0].jobType == model.JobTypePipelineReport &&
				decodeReport(t, b.got[0].payload).Outcome != model.MessageOutcomeNotTransaction {
				t.Fatalf("report %s", b.got[0].payload)
			}
		})
	}
}

type resultVerifier struct{ res ports.VerifyResult }

func (f resultVerifier) Verify(context.Context, ports.Message, ports.ExtractedEntities) (ports.VerifyResult, error) {
	return f.res, nil
}

func TestVerifyAddsWhatItRead(t *testing.T) {
	b := &fakeBroker{}
	deps := WorkerDeps{Broker: b, Pipeline: fakePipeline{verifier: resultVerifier{ports.VerifyResult{
		Valid: true, Confidence: 0.99, Channel: "UPI", VPA: "shop@oksbi", Ref: "900000000001", Account: "1111",
	}}}}
	j, _ := newJob(t, model.MessageEntitiesPayload{Message: testMessage, Entities: model.MessageEntities{Amount: 10}})

	if _, err := messageVerify(deps)(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	var next model.MessageEntitiesPayload
	if err := json.Unmarshal(b.got[0].payload, &next); err != nil {
		t.Fatal(err)
	}
	e := next.Entities
	if e.Channel != "UPI" || e.VPA != "shop@oksbi" || e.Ref != "900000000001" || e.Account != "1111" {
		t.Fatalf("entities %+v", e)
	}
}

func TestVerifyKeepsAnExtractedAccount(t *testing.T) {
	b := &fakeBroker{}
	deps := WorkerDeps{Broker: b, Pipeline: fakePipeline{verifier: resultVerifier{ports.VerifyResult{Valid: true, Confidence: 0.99, Account: "2222"}}}}
	j, _ := newJob(t, model.MessageEntitiesPayload{Message: testMessage, Entities: model.MessageEntities{Account: "1111"}})
	if _, err := messageVerify(deps)(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	var next model.MessageEntitiesPayload
	if err := json.Unmarshal(b.got[0].payload, &next); err != nil {
		t.Fatal(err)
	}
	if next.Entities.Account != "1111" {
		t.Fatalf("account %q", next.Entities.Account)
	}
}

func TestVerifyParks(t *testing.T) {
	date := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	complete := model.MessageEntities{Account: "1111", Merchant: "Shop", Amount: 10, Date: date}
	tests := []struct {
		name       string
		park       bool
		res        ports.VerifyResult
		entities   model.MessageEntities
		wantResume bool
	}{
		{"details don't match, parking off", false, ports.VerifyResult{Confidence: 0.2}, complete, true},
		{"unsure match", true, ports.VerifyResult{Valid: true, Confidence: 0.6}, complete, true},
		{"nothing to continue with: no amount", false, ports.VerifyResult{}, model.MessageEntities{Account: "1111", Date: date}, false},
		{"nothing to continue with: no date", false, ports.VerifyResult{}, model.MessageEntities{Account: "1111", Amount: 10}, false},
		{"nothing to continue with: no account", false, ports.VerifyResult{}, model.MessageEntities{Amount: 10, Date: date}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &fakeBroker{}
			deps := WorkerDeps{Broker: b, ParkLowConfidence: tt.park, Pipeline: fakePipeline{verifier: resultVerifier{tt.res}}}
			j, _ := newJob(t, model.MessageEntitiesPayload{Message: testMessage, Entities: tt.entities})
			if _, err := messageVerify(deps)(context.Background(), j); err != nil {
				t.Fatal(err)
			}
			report := decodeReport(t, b.got[0].payload)
			if report.Outcome != model.MessageOutcomeNeedsReview || report.Review == nil || report.Reason == "" {
				t.Fatalf("report %+v", report)
			}
			if report.Review.Step != "verify" || report.Review.Entities == nil || len(report.Review.Unsure) == 0 {
				t.Fatalf("review %+v", report.Review)
			}
			if got := report.Review.Resume != nil; got != tt.wantResume {
				t.Fatalf("resume = %v, want %v", got, tt.wantResume)
			}
			if tt.wantResume && report.Review.Resume.JobType != model.JobTypeMessageLookup {
				t.Fatalf("resume %+v", report.Review.Resume)
			}
		})
	}
}

func TestVerifyLowConfidenceOnlyLogsUntilParkingIsOn(t *testing.T) {
	b := &fakeBroker{}
	deps := WorkerDeps{Broker: b, Pipeline: fakePipeline{verifier: resultVerifier{ports.VerifyResult{Valid: true, Confidence: 0.6}}}}
	j, _ := newJob(t, model.MessageEntitiesPayload{Message: testMessage})
	if _, err := messageVerify(deps)(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	if b.got[0].jobType != model.JobTypeMessageLookup {
		t.Fatalf("queued %s", b.got[0].jobType)
	}
}

func TestMessageSourceComesFromEntities(t *testing.T) {
	msg := model.ReceivedMessage{Source: "sms", Body: "any wording at all"}
	when := time.Date(2026, 10, 7, 10, 30, 0, 0, time.UTC)
	got := messageSource(msg, model.MessageEntities{
		Account: "XX1234 5678", Merchant: "Blue Tokai", Amount: -450.5, Date: when, Currency: "inr",
		Channel: "UPI", VPA: "shop@oksbi", Ref: "900000000001",
	}, model.MessageVerdict{Type: "debit"})

	if got.Kind != "SMS" || got.Text != msg.Body || got.Channel != "UPI" || got.Merchant != "SHOP@OKSBI" ||
		got.VPA != "shop@oksbi" || got.Ref != "900000000001" || got.AccountHint != "5678" ||
		got.Direction != "DEBIT" || got.Amount != "450.50" || got.Currency != "INR" {
		t.Fatalf("source %+v", got)
	}
	if got.OccurredOn == nil || got.OccurredAt == nil {
		t.Fatalf("dates %v %v", got.OccurredOn, got.OccurredAt)
	}
}

func TestMessageSourceLeavesUnknownsBlank(t *testing.T) {
	got := messageSource(model.ReceivedMessage{Source: "api", Body: "text"}, model.MessageEntities{
		Merchant: "Shop", Currency: "dollars", Date: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC),
	}, model.MessageVerdict{Type: "none"})
	if got.Kind != "" || got.Channel != "" || got.Direction != "" || got.Amount != "" || got.Currency != "" || got.OccurredAt != nil {
		t.Fatalf("source %+v", got)
	}
}

func TestDisplayTextKeepsLineBreaks(t *testing.T) {
	msg := model.ReceivedMessage{Source: "email", Body: "<p>Rs.10 debited.</p><p>Call us.</p>"}
	if got := displayText(msg); !strings.Contains(got, "\n") || strings.Contains(got, "<p>") {
		t.Fatalf("display text %q", got)
	}
	if got := plainBody(msg); strings.Contains(got, "\n") {
		t.Fatalf("hash text changed: %q", got)
	}
}

func TestMessageSourceReadsTheClockInTheBanksZone(t *testing.T) {
	// Extract keeps "20:41:11" from the message as 20:41:11 UTC.
	stated := time.Date(2026, 11, 10, 20, 41, 11, 0, time.UTC)
	got := messageSource(model.ReceivedMessage{Source: "sms"}, model.MessageEntities{Date: stated}, model.MessageVerdict{})
	want := time.Date(2026, 11, 10, 15, 11, 11, 0, time.UTC) // 20:41:11 IST
	if got.OccurredAt == nil || !got.OccurredAt.Equal(want) {
		t.Fatalf("occurred at %v, want %v", got.OccurredAt, want)
	}
	if got.OccurredOn == nil || got.OccurredOn.Format(time.DateOnly) != "2026-11-10" {
		t.Fatalf("occurred on %v", got.OccurredOn)
	}
}

func TestCurrency(t *testing.T) {
	for raw, want := range map[string]string{"Rs": "INR", "Rs.": "INR", "₹": "INR", "inr": "INR", " usd ": "USD", "dollars": "", "": ""} {
		if got := currency(raw); got != want {
			t.Errorf("currency(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestVerifyPassesItsScoreOn(t *testing.T) {
	b := &fakeBroker{}
	deps := WorkerDeps{Broker: b, Pipeline: fakePipeline{verifier: resultVerifier{ports.VerifyResult{Valid: true, Confidence: 0.98, Model: "jev"}}}}
	j, _ := newJob(t, model.MessageEntitiesPayload{Message: testMessage, Entities: model.MessageEntities{Amount: 10}})
	if _, err := messageVerify(deps)(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	var next model.MessageEntitiesPayload
	if err := json.Unmarshal(b.got[0].payload, &next); err != nil {
		t.Fatal(err)
	}
	if next.Entities.VerifiedConfidence == nil || *next.Entities.VerifiedConfidence != 0.98 || next.Entities.VerifiedBy != "jev" {
		t.Fatalf("entities %+v", next.Entities)
	}
}

func TestCreatePayloadCarriesTheReadableText(t *testing.T) {
	b := &fakeBroker{}
	deps := WorkerDeps{Broker: b, Pipeline: fakePipeline{memory: fakeMemory{found: true}}}
	msg := model.ReceivedMessage{Source: "email", Body: "<p>Rs.10 debited.</p><p>Call us.</p>"}
	j, _ := newJob(t, model.MessageEntitiesPayload{Message: msg})
	if _, err := messageLookup(deps)(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	var p model.TransactionCreatePayload
	if err := json.Unmarshal(b.got[0].payload, &p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.DisplayText, "\n") || p.Source == nil || p.Source.Kind != "EMAIL" {
		t.Fatalf("payload %+v", p)
	}
}

func TestModelNamesReachThePrediction(t *testing.T) {
	sure := 0.9
	verdict := toModelVerdict(ports.TxnVerdict{Usage: &ports.Usage{Model: "jev"}})
	prediction := toModelPrediction(ports.PredictionResult{PayeeConfidence: &sure, Reasoning: "r", Model: "haiku"}, model.MessagePredictionPredictor)
	if verdict.Model != "jev" || prediction.Model != "haiku" || prediction.Reasoning != "r" || *prediction.PayeeConfidence != 0.9 {
		t.Fatalf("verdict %+v, prediction %+v", verdict, prediction)
	}
}
