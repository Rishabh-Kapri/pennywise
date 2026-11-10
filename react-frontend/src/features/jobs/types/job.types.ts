import type { LoadingState } from '@/utils';

export type JobStatus = 'queued' | 'running' | 'succeeded' | 'cancelled' | 'dead';

export interface JobProgress {
  message?: string;
  current?: number;
  total?: number;
}

/** A websocket notice that a job changed. */
export interface JobUpdate {
  jobId: string;
  status: JobStatus;
  progress?: JobProgress;
}

export interface JobSummary {
  id: string;
  parentId?: string;
  type: string;
  queue: string;
  status: JobStatus;
  attempt: number;
  maxAttempts: number;
  /** The subject, mailbox address, or how a message ended. */
  label?: string;
  /** Where a message.received job came from: email, sms or api. */
  source?: string;
  /** What it's doing while it runs. Empty until the job reports something. */
  progress?: JobProgress;
  lastError?: string;
  runAt: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
}

export interface JobTree {
  rootId: string;
  /** Every job under the root and the root itself, oldest first. */
  jobs: JobSummary[];
}

export interface JobCounts {
  running: number;
  queued: number;
  retrying: number;
  failed: number;
  /** Finished in the last 24 hours. */
  done: number;
}

export interface JobFailure {
  code: string;
  message: string;
  count: number;
  lastSeen: string;
}

export interface JobList {
  /** False when the API has no job store (JOB_DRIVER unset or memory). */
  enabled: boolean;
  counts: JobCounts;
  failures: JobFailure[];
  trees: JobTree[];
  total: number;
}

export interface JobEvent {
  type: 'enqueued' | 'claimed' | 'retry_scheduled' | 'reaped' | 'completed' | 'dead' | 'cancelled' | 'manual_retry';
  status: JobStatus;
  attempt: number;
  actor: string;
  error?: string;
  reason?: string;
  durationMs?: number;
  nextRunAt?: string;
  createdAt: string;
}

export interface JobDetail extends JobSummary {
  correlationId?: string;
  originService?: string;
  lockedBy?: string;
  payload?: unknown;
  result?: unknown;
  events: JobEvent[];
}

/** What a person needs to decide about a message parked for review. */
export interface MessageReview {
  step: string;
  /** The message as a person reads it, as plain text. */
  text: string;
  entities?: {
    account?: string;
    merchant?: string;
    amount?: number;
    date?: string;
    currency?: string;
    channel?: string;
    vpa?: string;
    ref?: string;
  };
  /** Which of amount, account, date and merchant to double-check. */
  unsure?: string[];
  /** Set when the message has what a transaction needs, so it can continue. */
  resume?: unknown;
}

/** The result a message pipeline step stores. */
export interface MessageStepResult {
  /** What the step produced. Jobs from before this was recorded don't have it. */
  output?: unknown;
  nextJobType?: string;
  nextJobId?: string;
  outcome?: string;
}

export interface JobFilters {
  errorCode: string;
  jobId: string;
  /** How many of the newest syncs to load. Grows until the days on screen are complete. */
  limit: number;
}

export interface JobsState {
  list: JobList | null;
  listLoading: LoadingState;
  error: string | null;
  filters: JobFilters;
  /** When the list last loaded. */
  updatedAt: string | null;
  /** Goes up when a job changes, which prompts the open screen to reload. */
  liveVersion: number;
  /** Jobs opened on this screen, by ID. */
  details: Record<string, JobDetail>;
  detailLoading: LoadingState;
  /** The job a retry or cancel is running for. */
  actingJobId: string | null;
}
