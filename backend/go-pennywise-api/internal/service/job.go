package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

const (
	defaultJobTreeLimit = 30
	maxJobTreeLimit     = 1000
	jobDoneWindow       = 24 * time.Hour
)

// JobListInput is the Jobs screen's filter. Status also accepts "retrying".
type JobListInput struct {
	Status    string
	Type      string
	Queue     string
	ErrorCode string
	JobID     *uuid.UUID
	Since     *time.Time
	Limit     int
	Offset    int
}

// JobSummary is one job in a tree, without its payload. Label is a short
// description for the list: the message's subject or the mailbox address.
type JobSummary struct {
	ID          uuid.UUID       `json:"id"`
	ParentID    *uuid.UUID      `json:"parentId,omitempty"`
	Type        string          `json:"type"`
	Queue       string          `json:"queue"`
	Status      job.JobStatus   `json:"status"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"maxAttempts"`
	Label       string          `json:"label,omitempty"`
	Source      string          `json:"source,omitempty"` // message.received only: email, sms or api
	Progress    json.RawMessage `json:"progress,omitempty"`
	LastError   string          `json:"lastError,omitempty"`
	RunAt       time.Time       `json:"runAt"`
	StartedAt   *time.Time      `json:"startedAt,omitempty"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// JobTreeView is a top-level job and everything under it, oldest first.
type JobTreeView struct {
	RootID uuid.UUID    `json:"rootId"`
	Jobs   []JobSummary `json:"jobs"`
}

type JobCountsView struct {
	Running  int `json:"running"`
	Queued   int `json:"queued"`
	Retrying int `json:"retrying"`
	Failed   int `json:"failed"`
	Done     int `json:"done"` // in the last 24 hours
}

type JobFailureView struct {
	Code     string    `json:"code"`
	Message  string    `json:"message"`
	Count    int       `json:"count"`
	LastSeen time.Time `json:"lastSeen"`
}

// JobList is the Jobs screen. Enabled is false when this service has no job
// store (JOB_DRIVER is unset or memory).
type JobList struct {
	Enabled  bool             `json:"enabled"`
	Counts   JobCountsView    `json:"counts"`
	Failures []JobFailureView `json:"failures"`
	Trees    []JobTreeView    `json:"trees"`
	Total    int              `json:"total"`
}

type JobEventView struct {
	Type       job.JobEventType `json:"type"`
	Status     job.JobStatus    `json:"status"`
	Attempt    int              `json:"attempt"`
	Actor      string           `json:"actor"`
	Error      string           `json:"error,omitempty"`
	Reason     string           `json:"reason,omitempty"`
	DurationMs *int64           `json:"durationMs,omitempty"`
	NextRunAt  *time.Time       `json:"nextRunAt,omitempty"`
	CreatedAt  time.Time        `json:"createdAt"`
}

// JobDetail is one job in full, for the details panel.
type JobDetail struct {
	JobSummary
	CorrelationID string          `json:"correlationId,omitempty"`
	OriginService string          `json:"originService,omitempty"`
	LockedBy      string          `json:"lockedBy,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Result        json.RawMessage `json:"result,omitempty"`
	Events        []JobEventView  `json:"events"`
}

// JobService shows the budget's jobs. The budget comes from ctx.
type JobService interface {
	List(ctx context.Context, input JobListInput) (JobList, error)
	// Get, Retry and Cancel return CodeJobNotFound for a job the budget can't
	// see. Retry and Cancel return CodeJobInvalidState when the job isn't dead
	// or queued, respectively.
	Get(ctx context.Context, id uuid.UUID) (JobDetail, error)
	Retry(ctx context.Context, id uuid.UUID) error
	Cancel(ctx context.Context, id uuid.UUID) error
	// RetryFailed queues every dead job with this error code again and returns
	// how many it queued.
	RetryFailed(ctx context.Context, errorCode string) (int, error)
	// RunAgain sends a message the pipeline stopped through it again, replacing
	// the stopped run. CodeJobInvalidState unless it's such a message.
	RunAgain(ctx context.Context, id uuid.UUID) error
	// Continue carries on with a message parked for review, from the step after
	// the one that parked it. CodeJobInvalidState unless it's parked and has
	// what a transaction needs.
	Continue(ctx context.Context, id uuid.UUID) error
	// Dismiss hides a finished job and what followed it. CodeJobInvalidState
	// while any of them is still running, or for a job that fanned out into
	// several messages.
	Dismiss(ctx context.Context, id uuid.UUID) error
}

type jobService struct {
	store       job.JobStore // nil when JOB_DRIVER is unset or memory
	broker      job.Broker
	connections repository.MailConnectionRepository
}

// hiddenRootJobTypes are jobs that only add noise as their own entry in the
// list. Inside a message's pipeline they're still shown.
var hiddenRootJobTypes = []string{model.JobTypeTransactionLearn}

func NewJobService(store job.JobStore, broker job.Broker, connections repository.MailConnectionRepository) JobService {
	return &jobService{store: store, broker: broker, connections: connections}
}

// scope reads the budget from ctx and finds the mailboxes feeding it.
func (s *jobService) scope(ctx context.Context) (job.JobScope, error) {
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return job.JobScope{}, err
	}
	connections, err := s.connections.ListByBudget(ctx, budgetID)
	if err != nil {
		return job.JobScope{}, err
	}
	scope := job.JobScope{BudgetID: budgetID, Accounts: make([]string, 0, len(connections))}
	for _, c := range connections {
		scope.Accounts = append(scope.Accounts, strings.ToLower(c.Account))
	}
	return scope, nil
}

func (s *jobService) List(ctx context.Context, input JobListInput) (JobList, error) {
	list := JobList{Failures: []JobFailureView{}, Trees: []JobTreeView{}}
	if s.store == nil {
		return list, nil
	}
	list.Enabled = true

	scope, err := s.scope(ctx)
	if err != nil {
		return list, err
	}

	counts, failures, err := s.store.Overview(ctx, scope, time.Now().Add(-jobDoneWindow))
	if err != nil {
		return list, err
	}
	list.Counts = JobCountsView(counts)
	for _, f := range failures {
		list.Failures = append(list.Failures, JobFailureView(f))
	}

	limit := input.Limit
	if limit <= 0 {
		limit = defaultJobTreeLimit
	}
	filter := job.JobTreeFilter{
		Scope:     scope,
		Type:      input.Type,
		Queue:     input.Queue,
		ErrorCode: input.ErrorCode,
		JobID:     input.JobID,
		Since:     input.Since,
		Limit:     min(limit, maxJobTreeLimit),
		Offset:    input.Offset,
	}
	// A failing hidden job still has to be reachable from "Failing now" or by ID.
	if input.ErrorCode == "" && input.JobID == nil {
		filter.HiddenRootTypes = hiddenRootJobTypes
		filter.HideEmptySyncs = true
	}
	if input.Status == "retrying" {
		filter.Retrying = true
	} else {
		filter.Status = job.JobStatus(input.Status)
	}

	trees, total, err := s.store.ListTrees(ctx, filter)
	if err != nil {
		return list, err
	}
	list.Total = total
	for _, tree := range trees {
		view := JobTreeView{RootID: tree.Root.ID, Jobs: make([]JobSummary, 0, len(tree.Jobs))}
		for i := range tree.Jobs {
			view.Jobs = append(view.Jobs, summarizeJob(&tree.Jobs[i]))
		}
		list.Trees = append(list.Trees, view)
	}
	return list, nil
}

func (s *jobService) Get(ctx context.Context, id uuid.UUID) (JobDetail, error) {
	j, err := s.visibleJob(ctx, id)
	if err != nil {
		return JobDetail{}, err
	}
	events, err := s.store.ListEvents(ctx, id)
	if err != nil {
		return JobDetail{}, err
	}

	detail := JobDetail{
		JobSummary:    summarizeJob(j),
		CorrelationID: j.Metadata.CorrelationID,
		OriginService: j.Metadata.OriginService,
		Payload:       j.Payload,
		Result:        j.Result,
		Events:        make([]JobEventView, 0, len(events)),
	}
	if j.LockedBy != nil {
		detail.LockedBy = *j.LockedBy
	}
	for _, e := range events {
		detail.Events = append(detail.Events, JobEventView{
			Type:       e.Type,
			Status:     e.Status,
			Attempt:    e.Attempt,
			Actor:      e.Actor,
			Error:      e.Error,
			Reason:     e.Reason,
			DurationMs: e.DurationMs,
			NextRunAt:  e.NextRunAt,
			CreatedAt:  e.CreatedAt,
		})
	}
	return detail, nil
}

func (s *jobService) Retry(ctx context.Context, id uuid.UUID) error {
	if _, err := s.visibleJob(ctx, id); err != nil {
		return err
	}
	_, err := s.store.RetryDead(ctx, id, jobActor(ctx))
	return err
}

func (s *jobService) Cancel(ctx context.Context, id uuid.UUID) error {
	if _, err := s.visibleJob(ctx, id); err != nil {
		return err
	}
	_, err := s.store.CancelJob(ctx, id, jobActor(ctx))
	return err
}

// maxRetryFailed bounds one "retry all", which runs in a single transaction.
const maxRetryFailed = 500

func (s *jobService) RetryFailed(ctx context.Context, errorCode string) (int, error) {
	if s.store == nil {
		return 0, errs.New(errs.CodeJobNotFound, "job not found")
	}
	scope, err := s.scope(ctx)
	if err != nil {
		return 0, err
	}
	return s.store.RetryDeadByCode(ctx, scope, errorCode, jobActor(ctx), maxRetryFailed)
}

func (s *jobService) RunAgain(ctx context.Context, id uuid.UUID) error {
	j, err := s.visibleJob(ctx, id)
	if err != nil {
		return err
	}
	if s.broker == nil || j.BudgetID == nil || j.Type != model.JobTypeMessageReceived {
		return errs.New(errs.CodeJobInvalidState, "only a message can run again")
	}
	chain, err := s.chain(ctx, j)
	if err != nil {
		return err
	}
	if !stoppedMessage(chain) {
		return errs.New(errs.CodeJobInvalidState, "the message didn't stop in the pipeline")
	}

	var msg model.ReceivedMessage
	if err := json.Unmarshal(j.Payload, &msg); err != nil {
		return errs.Wrap(errs.CodeJobInvalidPayload, "decode message", err)
	}
	// The stopped job's ID: a double click queues one run, not two.
	msg.RunID = j.ID.String()
	payload, err := json.Marshal(msg)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "encode message", err)
	}
	if _, err := s.broker.Enqueue(ctx, j.Type, payload,
		job.WithQueue(j.Queue),
		job.WithBudgetID(j.BudgetID),
		job.WithIdempotencyKey(msg.IdempotencyKey(*j.BudgetID)),
		job.WithRootJobID(j.RootJobID),
		job.WithParentID(j.ParentID),
	); err != nil {
		return err
	}
	return s.store.Dismiss(ctx, jobIDs(chain))
}

func (s *jobService) Continue(ctx context.Context, id uuid.UUID) error {
	j, err := s.visibleJob(ctx, id)
	if err != nil {
		return err
	}
	if s.broker == nil || j.BudgetID == nil || j.Type != model.JobTypeMessageReceived {
		return errs.New(errs.CodeJobInvalidState, "only a message can continue")
	}
	chain, err := s.chain(ctx, j)
	if err != nil {
		return err
	}
	report, resume := parkedReport(chain)
	if report == nil || resume == nil {
		return errs.New(errs.CodeJobInvalidState, "the message isn't waiting for review")
	}
	if _, err := s.broker.Enqueue(ctx, resume.JobType, resume.Payload,
		job.WithQueue(resume.Queue),
		job.WithBudgetID(j.BudgetID),
		job.WithIdempotencyKey(resume.IdempotencyKey),
		job.WithRootJobID(report.RootJobID),
		job.WithParentID(report.ParentID),
	); err != nil {
		return err
	}
	// The run goes on from the step that parked, so its report is over.
	return s.store.Dismiss(ctx, []uuid.UUID{report.ID})
}

// parkedReport finds the report of a message waiting for review, and the job
// that carries it on (nil if it can't be continued). The report is nil when
// the message isn't waiting: all its jobs must have finished.
func parkedReport(chain []job.Job) (*job.Job, *model.MessageResume) {
	var parked *job.Job
	var resume *model.MessageResume
	for i := range chain {
		if chain[i].Status != job.SucceededJobStatus {
			return nil, nil
		}
		if chain[i].Type != model.JobTypePipelineReport {
			continue
		}
		var report model.PipelineReportPayload
		if err := json.Unmarshal(chain[i].Payload, &report); err != nil {
			return nil, nil
		}
		parked, resume = nil, nil
		if report.Outcome == model.MessageOutcomeNeedsReview && report.Review != nil {
			parked, resume = &chain[i], report.Review.Resume
		}
	}
	return parked, resume
}

func (s *jobService) Dismiss(ctx context.Context, id uuid.UUID) error {
	j, err := s.visibleJob(ctx, id)
	if err != nil {
		return err
	}
	chain, err := s.chain(ctx, j)
	if err != nil {
		return err
	}
	for i := range chain {
		fannedOut := chain[i].ID != j.ID && chain[i].Type == model.JobTypeMessageReceived
		active := chain[i].Status == job.QueuedJobStatus || chain[i].Status == job.RunningJobStatus
		if fannedOut || active {
			return errs.New(errs.CodeJobInvalidState, "this job can't be dismissed")
		}
	}
	return s.store.Dismiss(ctx, jobIDs(chain))
}

// chain returns j and every job that followed from it.
func (s *jobService) chain(ctx context.Context, j *job.Job) ([]job.Job, error) {
	rootID := j.ID
	if j.RootJobID != nil {
		rootID = *j.RootJobID
	}
	family, err := s.store.ListByRoot(ctx, rootID)
	if err != nil {
		return nil, err
	}
	children := make(map[uuid.UUID][]job.Job, len(family))
	for i := range family {
		if family[i].ParentID != nil {
			children[*family[i].ParentID] = append(children[*family[i].ParentID], family[i])
		}
	}
	chain := []job.Job{*j}
	for i := 0; i < len(chain); i++ {
		chain = append(chain, children[chain[i].ID]...)
	}
	return chain, nil
}

func jobIDs(jobs []job.Job) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(jobs))
	for i := range jobs {
		ids = append(ids, jobs[i].ID)
	}
	return ids
}

// stoppedMessage reports whether a message's jobs all finished and the pipeline
// ended it without the transaction it should have had.
func stoppedMessage(chain []job.Job) bool {
	stopped := false
	for i := range chain {
		if chain[i].Status != job.SucceededJobStatus {
			return false
		}
		if chain[i].Type != model.JobTypePipelineReport {
			continue
		}
		var report model.PipelineReportPayload
		if err := json.Unmarshal(chain[i].Payload, &report); err != nil {
			return false
		}
		stopped = report.Outcome == model.MessageOutcomeUnverified || report.Outcome == model.MessageOutcomeNoPrediction
	}
	return stopped
}

// visibleJob loads a job the budget may see, applying the same rule as
// ListTrees: the budget's own job, one of its mailboxes' system jobs, or a
// system job that led to one of its jobs.
func (s *jobService) visibleJob(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	notFound := errs.New(errs.CodeJobNotFound, "job not found")
	if s.store == nil {
		return nil, notFound
	}
	scope, err := s.scope(ctx)
	if err != nil {
		return nil, err
	}
	j, err := s.store.GetJobByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if j.BudgetID != nil {
		if *j.BudgetID != scope.BudgetID {
			return nil, notFound
		}
		return j, nil
	}
	if account := mailboxAccount(j); account != "" && slices.Contains(scope.Accounts, account) {
		return j, nil
	}

	rootID := j.ID
	if j.RootJobID != nil {
		rootID = *j.RootJobID
	}
	family, err := s.store.ListByRoot(ctx, rootID)
	if err != nil {
		return nil, err
	}
	for i := range family {
		if family[i].BudgetID != nil && *family[i].BudgetID == scope.BudgetID {
			return j, nil
		}
	}
	return nil, notFound
}

func jobActor(ctx context.Context) string {
	if userID, err := utils.UserIDFromContext(ctx); err == nil {
		return "user:" + userID.String()
	}
	return "user"
}

// mailboxAccount returns the lowercase address of a mailbox.changed job, or "".
func mailboxAccount(j *job.Job) string {
	if j.Type != model.JobTypeMailboxChanged {
		return ""
	}
	var p model.MailboxChanged
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return ""
	}
	return strings.ToLower(p.Account)
}

func summarizeJob(j *job.Job) JobSummary {
	return JobSummary{
		ID:          j.ID,
		ParentID:    j.ParentID,
		Type:        j.Type,
		Queue:       j.Queue,
		Status:      j.Status,
		Attempt:     j.Attempt,
		MaxAttempts: j.MaxAttempts,
		Label:       jobLabel(j),
		Source:      messageSource(j),
		Progress:    j.Progress,
		LastError:   j.LastError,
		RunAt:       j.RunAt,
		StartedAt:   j.StartedAt,
		CompletedAt: j.CompletedAt,
		CreatedAt:   j.CreatedAt,
	}
}

// messageSource says where a message.received job came from, or "".
func messageSource(j *job.Job) string {
	if j.Type != model.JobTypeMessageReceived {
		return ""
	}
	var p model.ReceivedMessage
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return ""
	}
	return p.Source
}

// jobLabel names a job in the list: what a message or mailbox job is about, and
// how a message ended. The other steps sit under their message's label.
func jobLabel(j *job.Job) string {
	switch j.Type {
	case model.JobTypeMailboxChanged:
		var p model.MailboxChanged
		if err := json.Unmarshal(j.Payload, &p); err == nil {
			return p.Account
		}
	case model.JobTypeMailboxBackfill:
		var p model.MailboxBackfill
		if err := json.Unmarshal(j.Payload, &p); err == nil {
			return p.Account
		}
	case model.JobTypeMessageReceived:
		var p model.ReceivedMessage
		if err := json.Unmarshal(j.Payload, &p); err == nil {
			if p.Subject != "" {
				return p.Subject
			}
			return p.Sender
		}
	case model.JobTypePipelineReport:
		var p model.PipelineReportPayload
		if err := json.Unmarshal(j.Payload, &p); err == nil {
			return string(p.Outcome)
		}
	}
	return ""
}
