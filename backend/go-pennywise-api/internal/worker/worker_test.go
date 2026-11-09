package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

type fakeBroker struct {
	job.Broker
	jobType string
	queue   string
	payload []byte
	err     error
}

func (b *fakeBroker) Enqueue(
	_ context.Context, jobType string, payload []byte, opts ...job.EnqueueOption,
) (job.EnqueueResult, error) {
	o, err := job.ResolveEnqueueOptions(jobType, payload, opts...)
	if err != nil {
		return job.EnqueueResult{}, err
	}
	b.jobType, b.queue, b.payload = jobType, o.Queue, payload
	return job.EnqueueResult{JobID: uuid.New()}, b.err
}

type fakeMessages struct {
	result    model.TransactionCreateResult
	err       error
	reported  *model.PipelineReportPayload
	learned   *model.TransactionLearnPayload
	gotBudget uuid.UUID
}

func (f *fakeMessages) CreateTransaction(
	ctx context.Context, _ model.TransactionCreatePayload,
) (model.TransactionCreateResult, error) {
	f.gotBudget, _ = utils.BudgetIDFromContext(ctx)
	return f.result, f.err
}

func (f *fakeMessages) LearnTransaction(ctx context.Context, p model.TransactionLearnPayload) error {
	f.gotBudget, _ = utils.BudgetIDFromContext(ctx)
	f.learned = &p
	return f.err
}

func (f *fakeMessages) ReportPipeline(ctx context.Context, p model.PipelineReportPayload) error {
	f.gotBudget, _ = utils.BudgetIDFromContext(ctx)
	f.reported = &p
	return f.err
}

func newJob(t *testing.T, payload any) (*job.Job, uuid.UUID) {
	t.Helper()
	p, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	budget := uuid.New()
	return &job.Job{ID: uuid.New(), Type: "test", BudgetID: &budget, Payload: p}, budget
}

func TestTransactionCreateQueuesReport(t *testing.T) {
	txnID := uuid.New()
	b := &fakeBroker{}
	m := &fakeMessages{result: model.TransactionCreateResult{TransactionIDs: []uuid.UUID{txnID}}}
	j, budget := newJob(t, model.TransactionCreatePayload{})

	if _, err := transactionCreate(Deps{Broker: b, Messages: m})(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	if m.gotBudget != budget {
		t.Fatalf("service saw budget %s", m.gotBudget)
	}
	if b.jobType != model.JobTypePipelineReport || b.queue != model.QueuePennywise {
		t.Fatalf("queued %s on %s", b.jobType, b.queue)
	}
	var report model.PipelineReportPayload
	if err := json.Unmarshal(b.payload, &report); err != nil {
		t.Fatal(err)
	}
	if report.Outcome != model.MessageOutcomeTransactionCreated || len(report.TransactionIDs) != 1 ||
		report.TransactionIDs[0] != txnID {
		t.Fatalf("%+v", report)
	}
}

func TestTransactionCreateFailureQueuesNothing(t *testing.T) {
	b := &fakeBroker{}
	m := &fakeMessages{err: errs.New(errs.CodeTransactionCreateFailed, "no")}
	j, _ := newJob(t, model.TransactionCreatePayload{})

	_, err := transactionCreate(Deps{Broker: b, Messages: m})(context.Background(), j)
	if !errs.HasCode(err, errs.CodeTransactionCreateFailed) {
		t.Fatalf("got %v", err)
	}
	if b.jobType != "" {
		t.Fatalf("queued %s", b.jobType)
	}
}

func TestPipelineReport(t *testing.T) {
	m := &fakeMessages{}
	j, budget := newJob(t, model.PipelineReportPayload{Outcome: model.MessageOutcomeUnverified})

	if _, err := pipelineReport(Deps{Messages: m})(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	if m.reported == nil || m.reported.Outcome != model.MessageOutcomeUnverified || m.gotBudget != budget {
		t.Fatalf("%+v budget %s", m.reported, m.gotBudget)
	}

	m.err = errors.New("boom")
	if _, err := pipelineReport(Deps{Messages: m})(context.Background(), j); !errors.Is(err, m.err) {
		t.Fatalf("got %v", err)
	}
}

func TestBadJobs(t *testing.T) {
	deps := Deps{Broker: &fakeBroker{}, Messages: &fakeMessages{}}

	noBudget, _ := newJob(t, model.TransactionCreatePayload{})
	noBudget.BudgetID = nil
	if _, err := transactionCreate(deps)(context.Background(), noBudget); !errs.HasCode(err, errs.CodeJobInvalidPayload) {
		t.Fatalf("no budget: %v", err)
	}

	badPayload, _ := newJob(t, model.PipelineReportPayload{})
	badPayload.Payload = []byte("nope")
	if _, err := pipelineReport(deps)(context.Background(), badPayload); !errs.HasCode(err, errs.CodeJobInvalidPayload) {
		t.Fatalf("bad payload: %v", err)
	}
}

func TestPipelineReportQueuesLearning(t *testing.T) {
	b := &fakeBroker{}
	txn := uuid.New()
	j, _ := newJob(t, model.PipelineReportPayload{
		Outcome: model.MessageOutcomeTransactionCreated,
		Learn:   &model.TransactionLearnPayload{TransactionID: txn},
	})
	if _, err := pipelineReport(Deps{Broker: b, Messages: &fakeMessages{}})(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	if b.jobType != model.JobTypeTransactionLearn || b.queue != model.QueuePennywise {
		t.Fatalf("queued %s on %s", b.jobType, b.queue)
	}
}

func TestPipelineReportWithoutLearningQueuesNothing(t *testing.T) {
	b := &fakeBroker{}
	j, _ := newJob(t, model.PipelineReportPayload{Outcome: model.MessageOutcomeUnverified})
	if _, err := pipelineReport(Deps{Broker: b, Messages: &fakeMessages{}})(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	if b.jobType != "" {
		t.Fatalf("queued %s", b.jobType)
	}
}

func TestTransactionLearn(t *testing.T) {
	m := &fakeMessages{}
	txn := uuid.New()
	j, budget := newJob(t, model.TransactionLearnPayload{TransactionID: txn, Confirmed: true})
	if _, err := transactionLearn(Deps{Messages: m})(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	if m.gotBudget != budget || m.learned == nil || m.learned.TransactionID != txn || !m.learned.Confirmed {
		t.Fatalf("learned %+v for %s", m.learned, m.gotBudget)
	}
}

func TestTransactionCreateDuplicateIsReportedAndLearnsNothing(t *testing.T) {
	b := &fakeBroker{}
	txn := uuid.New()
	started := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	m := &fakeMessages{result: model.TransactionCreateResult{
		TransactionIDs: []uuid.UUID{txn},
		ExistingSince:  started.Add(-time.Hour),
		Learn:          &model.TransactionLearnPayload{TransactionID: txn},
	}}
	j, _ := newJob(t, model.TransactionCreatePayload{})
	j.CreatedAt = started

	raw, err := transactionCreate(Deps{Broker: b, Messages: m})(context.Background(), j)
	if err != nil {
		t.Fatal(err)
	}
	var report model.PipelineReportPayload
	if err := json.Unmarshal(b.payload, &report); err != nil {
		t.Fatal(err)
	}
	if report.Outcome != model.MessageOutcomeTransactionDuplicate || report.Learn != nil {
		t.Fatalf("%+v", report)
	}
	var result model.TransactionCreateResult
	if err := json.Unmarshal(raw, &result); err != nil || !result.Duplicate {
		t.Fatalf("result = %s, %v", raw, err)
	}
}

// A retry finding the row its own earlier attempt saved still counts as created.
func TestTransactionCreateRetryIsNotADuplicate(t *testing.T) {
	b := &fakeBroker{}
	txn := uuid.New()
	started := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	m := &fakeMessages{result: model.TransactionCreateResult{
		TransactionIDs: []uuid.UUID{txn},
		ExistingSince:  started.Add(time.Second),
		Learn:          &model.TransactionLearnPayload{TransactionID: txn},
	}}
	j, _ := newJob(t, model.TransactionCreatePayload{})
	j.CreatedAt = started

	if _, err := transactionCreate(Deps{Broker: b, Messages: m})(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	var report model.PipelineReportPayload
	if err := json.Unmarshal(b.payload, &report); err != nil {
		t.Fatal(err)
	}
	if report.Outcome != model.MessageOutcomeTransactionCreated || report.Learn == nil {
		t.Fatalf("%+v", report)
	}
}
