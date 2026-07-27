import type { LoadingState } from '@/utils';

export type PipelineRunStatus =
  | 'running'
  | 'waiting_retry'
  | 'completed'
  | 'failed';

export type PipelineStep =
  | 'fetch_user'
  | 'fetch_emails'
  | 'parse'
  | 'predict'
  | 'create_transactions'
  | 'done';

export type PipelineEventStatus =
  | 'started'
  | 'succeeded'
  | 'skipped'
  | 'failed'
  | 'waiting_retry'
  | 'retry_signaled';

export type PipelineTrigger = 'gmail_push' | 'manual';

export interface PipelineRun {
  id: string;
  budgetId: string;
  workflowId: string;
  workflowRunId: string;
  childWorkflowId?: string;
  trigger: PipelineTrigger;
  emailAccount?: string;
  /** Gmail history id that triggered the run; absent for manual runs. */
  gmailHistoryId?: number;
  status: PipelineRunStatus;
  currentStep: PipelineStep;
  error?: string;
  emailsFetched: number;
  emailsSkipped: number;
  transactionsCreated: number;
  startedAt: string;
  updatedAt: string;
  completedAt?: string;
}

export interface PipelineRunEvent {
  id: string;
  runId: string;
  step: PipelineStep;
  status: PipelineEventStatus;
  messageId?: string;
  detail?: Record<string, unknown>;
  createdAt: string;
}

export interface PipelineRunDetail {
  run: PipelineRun;
  events: PipelineRunEvent[];
}

export interface PipelineState {
  runs: PipelineRun[];
  runsLoading: LoadingState;
  error: string | null;
  eventsByRunId: Record<string, PipelineRunEvent[]>;
  detailLoading: LoadingState;
  retryingRunId: string | null;
}
