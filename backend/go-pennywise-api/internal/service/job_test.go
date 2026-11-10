package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

type fakeJobStore struct {
	job.JobStore
	jobs     map[uuid.UUID]*job.Job
	family   []job.Job
	filter   job.JobTreeFilter
	scope    job.JobScope
	retried  []uuid.UUID
	canceled []uuid.UUID
	actor    string

	retriedCode  string
	retriedLimit int
	dismissed    []uuid.UUID
}

func (f *fakeJobStore) GetJobByID(_ context.Context, id uuid.UUID) (*job.Job, error) {
	if j, ok := f.jobs[id]; ok {
		return j, nil
	}
	return nil, errs.New(errs.CodeJobNotFound, "job not found")
}

func (f *fakeJobStore) ListByRoot(context.Context, uuid.UUID) ([]job.Job, error) {
	return f.family, nil
}

func (f *fakeJobStore) ListEvents(context.Context, uuid.UUID) ([]job.JobEvent, error) {
	return []job.JobEvent{{Type: job.EventDead, Attempt: 3, Error: "boom"}}, nil
}

func (f *fakeJobStore) ListTrees(_ context.Context, filter job.JobTreeFilter) ([]job.JobTree, int, error) {
	f.filter = filter
	var trees []job.JobTree
	for _, j := range f.jobs {
		trees = append(trees, job.JobTree{Root: *j, Jobs: []job.Job{*j}})
	}
	return trees, len(trees), nil
}

func (f *fakeJobStore) Overview(_ context.Context, scope job.JobScope, _ time.Time) (job.JobCounts, []job.JobFailure, error) {
	f.scope = scope
	return job.JobCounts{Failed: 2}, []job.JobFailure{{Code: "X", Count: 2}}, nil
}

func (f *fakeJobStore) RetryDead(_ context.Context, id uuid.UUID, actor string) (*job.Job, error) {
	f.retried, f.actor = append(f.retried, id), actor
	return nil, nil
}

func (f *fakeJobStore) RetryDeadByCode(_ context.Context, _ job.JobScope, code, actor string, limit int) (int, error) {
	f.retriedCode, f.actor, f.retriedLimit = code, actor, limit
	return 3, nil
}

func (f *fakeJobStore) Dismiss(_ context.Context, ids []uuid.UUID) error {
	f.dismissed = append(f.dismissed, ids...)
	return nil
}

func (f *fakeJobStore) CancelJob(_ context.Context, id uuid.UUID, actor string) (*job.Job, error) {
	f.canceled, f.actor = append(f.canceled, id), actor
	return nil, nil
}

type fakeConnections struct {
	repository.MailConnectionRepository
	accounts []string
}

func (f fakeConnections) ListByBudget(context.Context, uuid.UUID) ([]model.MailConnection, error) {
	out := make([]model.MailConnection, 0, len(f.accounts))
	for _, a := range f.accounts {
		out = append(out, model.MailConnection{Account: a})
	}
	return out, nil
}

func mailboxJob(account string) *job.Job {
	payload, _ := json.Marshal(model.MailboxChanged{Provider: "gmail", Account: account})
	return &job.Job{ID: uuid.New(), Type: model.JobTypeMailboxChanged, Payload: payload}
}

func TestJobListWithoutStore(t *testing.T) {
	list, err := NewJobService(nil, nil, nil).List(context.Background(), JobListInput{})
	if err != nil || list.Enabled || list.Trees == nil || list.Failures == nil {
		t.Fatalf("%+v, %v", list, err)
	}
}

func TestJobListScopeAndFilters(t *testing.T) {
	budget := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budget)
	message, _ := json.Marshal(model.ReceivedMessage{Subject: "Debit alert", Sender: "bank"})
	id := uuid.New()
	store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{
		id: {ID: id, Type: model.JobTypeMessageReceived, BudgetID: &budget, Payload: message},
	}}
	svc := NewJobService(store, nil, fakeConnections{accounts: []string{"Me@Gmail.com"}})

	list, err := svc.List(ctx, JobListInput{Status: "retrying", Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if !list.Enabled || list.Counts.Failed != 2 || len(list.Failures) != 1 || list.Total != 1 {
		t.Fatalf("%+v", list)
	}
	if got := list.Trees[0].Jobs[0].Label; got != "Debit alert" {
		t.Fatalf("label = %q", got)
	}
	if store.scope.BudgetID != budget || len(store.scope.Accounts) != 1 || store.scope.Accounts[0] != "me@gmail.com" {
		t.Fatalf("scope = %+v", store.scope)
	}
	if !store.filter.Retrying || store.filter.Status != "" || store.filter.Limit != maxJobTreeLimit {
		t.Fatalf("filter = %+v", store.filter)
	}

	if _, err := svc.List(ctx, JobListInput{Status: "dead"}); err != nil {
		t.Fatal(err)
	}
	if store.filter.Retrying || store.filter.Status != job.DeadJobStatus || store.filter.Limit != defaultJobTreeLimit {
		t.Fatalf("filter = %+v", store.filter)
	}
}

func TestJobListHidesLearnJobsUnlessLookingForOne(t *testing.T) {
	ctx := utils.WithBudgetID(context.Background(), uuid.New())
	store := &fakeJobStore{}
	svc := NewJobService(store, nil, fakeConnections{})

	if _, err := svc.List(ctx, JobListInput{}); err != nil {
		t.Fatal(err)
	}
	if len(store.filter.HiddenRootTypes) != 1 || store.filter.HiddenRootTypes[0] != model.JobTypeTransactionLearn ||
		!store.filter.HideEmptySyncs {
		t.Fatalf("filter = %+v", store.filter)
	}

	id := uuid.New()
	for name, input := range map[string]JobListInput{
		"failure": {ErrorCode: "TRANSACTION_LEARN_FAILED"},
		"job id":  {JobID: &id},
	} {
		if _, err := svc.List(ctx, input); err != nil {
			t.Fatal(err)
		}
		if len(store.filter.HiddenRootTypes) != 0 || store.filter.HideEmptySyncs {
			t.Fatalf("%s: filter = %+v", name, store.filter)
		}
	}
}

func TestSummarizeJobNamesTheMessageSource(t *testing.T) {
	sms := &job.Job{Type: model.JobTypeMessageReceived, Payload: []byte(`{"source":"sms","sender":"AX-HDFCBK"}`)}
	if got := summarizeJob(sms); got.Source != "sms" || got.Label != "AX-HDFCBK" {
		t.Fatalf("summary = %+v", got)
	}
	mailbox := &job.Job{Type: model.JobTypeMailboxChanged, Payload: []byte(`{"source":"sms"}`)}
	if got := summarizeJob(mailbox); got.Source != "" {
		t.Fatalf("mailbox job has a source: %+v", got)
	}
}

func TestJobVisibility(t *testing.T) {
	budget, other := uuid.New(), uuid.New()
	own := &job.Job{ID: uuid.New(), BudgetID: &budget, Payload: []byte(`{}`)}
	theirs := &job.Job{ID: uuid.New(), BudgetID: &other}
	mailbox := mailboxJob("ME@gmail.com")
	stranger := mailboxJob("stranger@gmail.com")
	detached := mailboxJob("old@gmail.com")
	detachedChild := job.Job{ID: uuid.New(), BudgetID: &budget, RootJobID: &detached.ID}
	strangerChild := job.Job{ID: uuid.New(), BudgetID: &other, RootJobID: &stranger.ID}

	userID := uuid.New()
	ctx := utils.WithUserID(utils.WithBudgetID(context.Background(), budget), userID)

	tests := []struct {
		name    string
		target  *job.Job
		family  []job.Job
		visible bool
	}{
		{"own job", own, nil, true},
		{"another budget's job", theirs, nil, false},
		{"own mailbox, any casing", mailbox, nil, true},
		{"unrelated mailbox", stranger, []job.Job{*stranger, strangerChild}, false},
		{"detached mailbox that led here", detached, []job.Job{*detached, detachedChild}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{tt.target.ID: tt.target}, family: tt.family}
			svc := NewJobService(store, nil, fakeConnections{accounts: []string{"me@gmail.com"}})

			_, getErr := svc.Get(ctx, tt.target.ID)
			retryErr := svc.Retry(ctx, tt.target.ID)
			cancelErr := svc.Cancel(ctx, tt.target.ID)

			if tt.visible {
				if getErr != nil || retryErr != nil || cancelErr != nil {
					t.Fatalf("%v, %v, %v", getErr, retryErr, cancelErr)
				}
				if len(store.retried) != 1 || len(store.canceled) != 1 || store.actor != "user:"+userID.String() {
					t.Fatalf("%+v", store)
				}
				return
			}
			for _, err := range []error{getErr, retryErr, cancelErr} {
				if !errs.HasCode(err, errs.CodeJobNotFound) {
					t.Fatalf("got %v", err)
				}
			}
			if len(store.retried)+len(store.canceled) != 0 {
				t.Fatalf("acted on a hidden job: %+v", store)
			}
		})
	}
}

func TestJobDetail(t *testing.T) {
	budget := uuid.New()
	worker := "cipher-worker-1"
	j := &job.Job{
		ID: uuid.New(), BudgetID: &budget, Type: model.JobTypeMessageExtract, Status: job.DeadJobStatus,
		Payload: []byte(`{"a":1}`), Result: []byte(`{"b":2}`), LockedBy: &worker,
		Metadata: job.Metadata{CorrelationID: "corr-1", OriginService: "ingest"},
	}
	store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{j.ID: j}}
	svc := NewJobService(store, nil, fakeConnections{})

	detail, err := svc.Get(utils.WithBudgetID(context.Background(), budget), j.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.CorrelationID != "corr-1" || detail.LockedBy != worker || string(detail.Payload) != `{"a":1}` ||
		string(detail.Result) != `{"b":2}` || len(detail.Events) != 1 || detail.Events[0].Error != "boom" {
		t.Fatalf("%+v", detail)
	}
}

type runAgainBroker struct {
	job.Broker
	jobType string
	payload []byte
	opts    job.EnqueueOptions
}

func (b *runAgainBroker) Enqueue(
	_ context.Context, jobType string, payload []byte, opts ...job.EnqueueOption,
) (job.EnqueueResult, error) {
	resolved, err := job.ResolveEnqueueOptions(jobType, payload, opts...)
	if err != nil {
		return job.EnqueueResult{}, err
	}
	b.jobType, b.payload, b.opts = jobType, payload, resolved
	return job.EnqueueResult{JobID: uuid.New()}, nil
}

// messageFamily is a mailbox job with one message under it, ended by outcome.
func messageFamily(budget uuid.UUID, outcome model.MessageOutcome) (root, msg job.Job, family []job.Job) {
	rootID := uuid.New()
	root = job.Job{ID: rootID, Type: model.JobTypeMailboxBackfill, BudgetID: &budget, Status: job.SucceededJobStatus}
	body, _ := json.Marshal(model.ReceivedMessage{Source: "email", Provider: "gmail", ExternalID: "m1"})
	msg = job.Job{
		ID: uuid.New(), Type: model.JobTypeMessageReceived, Queue: model.QueueCipher, BudgetID: &budget,
		RootJobID: &rootID, ParentID: &rootID, Status: job.SucceededJobStatus, Payload: body,
	}
	report, _ := json.Marshal(model.PipelineReportPayload{Outcome: outcome})
	end := job.Job{
		ID: uuid.New(), Type: model.JobTypePipelineReport, BudgetID: &budget, RootJobID: &rootID, ParentID: &msg.ID,
		Status: job.SucceededJobStatus, Payload: report,
	}
	return root, msg, []job.Job{root, msg, end}
}

func TestJobServiceRunAgain(t *testing.T) {
	budget := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budget)

	t.Run("replaces a stopped message with a new run", func(t *testing.T) {
		root, msg, family := messageFamily(budget, model.MessageOutcomeUnverified)
		store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{msg.ID: &msg}, family: family}
		broker := &runAgainBroker{}
		svc := NewJobService(store, broker, fakeConnections{})

		if err := svc.RunAgain(ctx, msg.ID); err != nil {
			t.Fatal(err)
		}
		var sent model.ReceivedMessage
		if err := json.Unmarshal(broker.payload, &sent); err != nil {
			t.Fatal(err)
		}
		if sent.RunID != msg.ID.String() || broker.jobType != model.JobTypeMessageReceived {
			t.Fatalf("queued %s %+v", broker.jobType, sent)
		}
		first := model.ReceivedMessage{Source: "email", Provider: "gmail", ExternalID: "m1"}.IdempotencyKey(budget)
		if broker.opts.IdempotencyKey != first+":run:"+msg.ID.String() || *broker.opts.RootJobID != root.ID {
			t.Fatalf("options = %+v", broker.opts)
		}
		if len(store.dismissed) != 2 {
			t.Fatalf("dismissed %v, want the message and its report", store.dismissed)
		}
	})

	t.Run("refuses a message that made its transaction", func(t *testing.T) {
		_, msg, family := messageFamily(budget, model.MessageOutcomeTransactionCreated)
		store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{msg.ID: &msg}, family: family}
		broker := &runAgainBroker{}
		err := NewJobService(store, broker, fakeConnections{}).RunAgain(ctx, msg.ID)
		if !errs.HasCode(err, errs.CodeJobInvalidState) || broker.jobType != "" {
			t.Fatalf("err = %v, queued %q", err, broker.jobType)
		}
	})
}

func TestJobServiceDismiss(t *testing.T) {
	budget := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budget)

	t.Run("hides a finished message", func(t *testing.T) {
		_, msg, family := messageFamily(budget, model.MessageOutcomeUnverified)
		store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{msg.ID: &msg}, family: family}
		if err := NewJobService(store, nil, fakeConnections{}).Dismiss(ctx, msg.ID); err != nil {
			t.Fatal(err)
		}
		if len(store.dismissed) != 2 {
			t.Fatalf("dismissed %v", store.dismissed)
		}
	})

	t.Run("refuses a job that fanned out, or one still running", func(t *testing.T) {
		root, msg, family := messageFamily(budget, model.MessageOutcomeUnverified)
		store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{root.ID: &root}, family: family}
		if err := NewJobService(store, nil, fakeConnections{}).Dismiss(ctx, root.ID); !errs.HasCode(err, errs.CodeJobInvalidState) {
			t.Fatalf("fan out: %v", err)
		}
		family[2].Status = job.RunningJobStatus
		store = &fakeJobStore{jobs: map[uuid.UUID]*job.Job{msg.ID: &msg}, family: family}
		if err := NewJobService(store, nil, fakeConnections{}).Dismiss(ctx, msg.ID); !errs.HasCode(err, errs.CodeJobInvalidState) {
			t.Fatalf("running: %v", err)
		}
		if len(store.dismissed) != 0 {
			t.Fatalf("dismissed %v", store.dismissed)
		}
	})
}

func TestJobServiceRetryFailed(t *testing.T) {
	ctx := utils.WithBudgetID(context.Background(), uuid.New())
	store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{}}
	got, err := NewJobService(store, nil, fakeConnections{}).RetryFailed(ctx, "MESSAGE_EXTRACT_FAILED")
	if err != nil || got != 3 || store.retriedCode != "MESSAGE_EXTRACT_FAILED" || store.retriedLimit != maxRetryFailed {
		t.Fatalf("got %d, %v, store %+v", got, err, store)
	}
}

// parkedFamily is a message parked for review at verify.
func parkedFamily(budget uuid.UUID, resume *model.MessageResume) (msg job.Job, report job.Job, family []job.Job) {
	root, msg, family := messageFamily(budget, model.MessageOutcomeNeedsReview)
	step := job.Job{
		ID: uuid.New(), Type: model.JobTypeMessageVerify, BudgetID: &budget, RootJobID: &root.ID, ParentID: &msg.ID,
		Status: job.SucceededJobStatus,
	}
	payload, _ := json.Marshal(model.PipelineReportPayload{
		Outcome: model.MessageOutcomeNeedsReview, Review: &model.MessageReview{Step: "verify", Resume: resume},
	})
	family[2].ParentID, family[2].Payload = &step.ID, payload
	family = []job.Job{family[0], family[1], step, family[2]}
	return msg, family[3], family
}

func TestJobServiceContinue(t *testing.T) {
	budget := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budget)
	resume := &model.MessageResume{
		JobType: model.JobTypeMessageLookup, Queue: model.QueueCipher, IdempotencyKey: "key:run:review:1", Payload: []byte(`{"a":1}`),
	}

	t.Run("queues the next step and removes only the report", func(t *testing.T) {
		msg, report, family := parkedFamily(budget, resume)
		store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{msg.ID: &msg}, family: family}
		broker := &runAgainBroker{}
		if err := NewJobService(store, broker, fakeConnections{}).Continue(ctx, msg.ID); err != nil {
			t.Fatal(err)
		}
		if broker.jobType != resume.JobType || string(broker.payload) != `{"a":1}` {
			t.Fatalf("queued %s %s", broker.jobType, broker.payload)
		}
		o := broker.opts
		if o.Queue != resume.Queue || o.IdempotencyKey != resume.IdempotencyKey || o.BudgetID == nil || *o.BudgetID != budget ||
			o.ParentID == nil || *o.ParentID != *report.ParentID || o.RootJobID == nil || *o.RootJobID != *report.RootJobID {
			t.Fatalf("options %+v", o)
		}
		if len(store.dismissed) != 1 || store.dismissed[0] != report.ID {
			t.Fatalf("dismissed %v, want only the report", store.dismissed)
		}
	})

	refused := func(t *testing.T, msg job.Job, family []job.Job) {
		t.Helper()
		store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{msg.ID: &msg}, family: family}
		broker := &runAgainBroker{}
		if err := NewJobService(store, broker, fakeConnections{}).Continue(ctx, msg.ID); !errs.HasCode(err, errs.CodeJobInvalidState) {
			t.Fatalf("err = %v", err)
		}
		if broker.jobType != "" || len(store.dismissed) != 0 {
			t.Fatalf("queued %q, dismissed %v", broker.jobType, store.dismissed)
		}
	}

	t.Run("refuses one with nothing to continue with", func(t *testing.T) {
		msg, _, family := parkedFamily(budget, nil)
		refused(t, msg, family)
	})
	t.Run("refuses one that's already moving", func(t *testing.T) {
		msg, _, family := parkedFamily(budget, resume)
		family[2].Status = job.QueuedJobStatus
		refused(t, msg, family)
	})
	t.Run("refuses a message that wasn't parked", func(t *testing.T) {
		_, msg, family := messageFamily(budget, model.MessageOutcomeUnverified)
		refused(t, msg, family)
	})
	t.Run("refuses another budget's message", func(t *testing.T) {
		msg, _, family := parkedFamily(uuid.New(), resume)
		store := &fakeJobStore{jobs: map[uuid.UUID]*job.Job{msg.ID: &msg}, family: family}
		if err := NewJobService(store, &runAgainBroker{}, fakeConnections{}).Continue(ctx, msg.ID); !errs.HasCode(err, errs.CodeJobNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}
