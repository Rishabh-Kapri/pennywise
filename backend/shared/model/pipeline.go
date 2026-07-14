package model

import (
	"time"

	"github.com/google/uuid"
)

// PipelineRunStatus is the overall state of an email-to-transaction pipeline run.
type PipelineRunStatus string

const (
	PipelineRunStatusRunning      PipelineRunStatus = "running"
	PipelineRunStatusWaitingRetry PipelineRunStatus = "waiting_retry"
	PipelineRunStatusCompleted    PipelineRunStatus = "completed"
	PipelineRunStatusFailed       PipelineRunStatus = "failed"
)

// PipelineStep identifies a stage of the email-to-transaction pipeline.
type PipelineStep string

const (
	PipelineStepFetchUser          PipelineStep = "fetch_user"
	PipelineStepFetchEmails        PipelineStep = "fetch_emails"
	PipelineStepParse              PipelineStep = "parse"
	PipelineStepPredict            PipelineStep = "predict"
	PipelineStepCreateTransactions PipelineStep = "create_transactions"
	PipelineStepDone               PipelineStep = "done"
)

// PipelineEventStatus is the state of a single timeline event within a run.
type PipelineEventStatus string

const (
	PipelineEventStarted       PipelineEventStatus = "started"
	PipelineEventSucceeded     PipelineEventStatus = "succeeded"
	PipelineEventSkipped       PipelineEventStatus = "skipped"
	PipelineEventFailed        PipelineEventStatus = "failed"
	PipelineEventWaitingRetry  PipelineEventStatus = "waiting_retry"
	PipelineEventRetrySignaled PipelineEventStatus = "retry_signaled"
)

// PipelineTrigger says how a run was started.
type PipelineTrigger string

const (
	PipelineTriggerGmailPush PipelineTrigger = "gmail_push"
	PipelineTriggerManual    PipelineTrigger = "manual"
)

// PipelineRun is the summary row for one workflow execution (pipeline_runs).
type PipelineRun struct {
	ID                  uuid.UUID         `json:"id"`
	BudgetID            uuid.UUID         `json:"budgetId"`
	WorkflowID          string            `json:"workflowId"`
	WorkflowRunID       string            `json:"workflowRunId"`
	ChildWorkflowID     *string           `json:"childWorkflowId,omitempty"`
	Trigger             PipelineTrigger   `json:"trigger"`
	EmailAccount        *string           `json:"emailAccount,omitempty"`
	Status              PipelineRunStatus `json:"status"`
	CurrentStep         PipelineStep      `json:"currentStep"`
	Error               *string           `json:"error,omitempty"`
	EmailsFetched       int               `json:"emailsFetched"`
	EmailsSkipped       int               `json:"emailsSkipped"`
	TransactionsCreated int               `json:"transactionsCreated"`
	StartedAt           time.Time         `json:"startedAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
	CompletedAt         *time.Time        `json:"completedAt,omitempty"`
}

// PipelineRunEvent is one append-only timeline entry (pipeline_run_events).
type PipelineRunEvent struct {
	ID        uuid.UUID           `json:"id"`
	RunID     uuid.UUID           `json:"runId"`
	Step      PipelineStep        `json:"step"`
	Status    PipelineEventStatus `json:"status"`
	MessageID *string             `json:"messageId,omitempty"`
	Detail    map[string]any      `json:"detail,omitempty"`
	CreatedAt time.Time           `json:"createdAt"`
}

// StartPipelineRunInput creates the pipeline_runs row. Executed as an activity
// once the budget is known; idempotent on (workflow_id, workflow_run_id).
type StartPipelineRunInput struct {
	BudgetID      uuid.UUID       `json:"budgetId"`
	WorkflowID    string          `json:"workflowId"`
	WorkflowRunID string          `json:"workflowRunId"`
	Trigger       PipelineTrigger `json:"trigger"`
	EmailAccount  string          `json:"emailAccount,omitempty"`
}

// PipelineEventInput is a timeline entry reported by the workflow.
type PipelineEventInput struct {
	Step      PipelineStep        `json:"step"`
	Status    PipelineEventStatus `json:"status"`
	MessageID string              `json:"messageId,omitempty"`
	Detail    map[string]any      `json:"detail,omitempty"`
}

// ReportPipelineStatusInput updates the run summary row and appends timeline
// events. Zero-valued run-level fields are left unchanged; Error uses a pointer
// so nil means "unchanged" and empty string means "clear".
type ReportPipelineStatusInput struct {
	RunID               uuid.UUID            `json:"runId"`
	BudgetID            uuid.UUID            `json:"budgetId"`
	RunStatus           PipelineRunStatus    `json:"runStatus,omitempty"`
	CurrentStep         PipelineStep         `json:"currentStep,omitempty"`
	ChildWorkflowID     string               `json:"childWorkflowId,omitempty"`
	Error               *string              `json:"error,omitempty"`
	EmailsFetched       *int                 `json:"emailsFetched,omitempty"`
	EmailsSkipped       *int                 `json:"emailsSkipped,omitempty"`
	TransactionsCreated *int                 `json:"transactionsCreated,omitempty"`
	Events              []PipelineEventInput `json:"events,omitempty"`
}

// PipelineRunDetail is the API response for a single run with its timeline.
type PipelineRunDetail struct {
	Run    PipelineRun        `json:"run"`
	Events []PipelineRunEvent `json:"events"`
}

// PipelineUpdateEvent is the websocket event name broadcast to a budget
// whenever a pipeline run row changes.
const PipelineUpdateEvent = "pennywise::pipeline::update"
