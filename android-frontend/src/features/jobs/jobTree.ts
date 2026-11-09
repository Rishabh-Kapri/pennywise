// Keep in step with react-frontend/src/features/jobs/jobTree.ts.
import type { JobEvent, JobStatus, JobSummary, JobTree } from './types';

/** The message pipeline, in order. `doing` is shown while the step runs. */
export const MESSAGE_STEPS: { type: string; label: string; doing: string }[] = [
  { type: 'message.received', label: 'Check', doing: 'Checking' },
  { type: 'message.extract', label: 'Extract', doing: 'Extracting' },
  { type: 'message.verify', label: 'Verify', doing: 'Verifying' },
  { type: 'message.lookup', label: 'Memory', doing: 'Looking up' },
  { type: 'message.predict', label: 'Predict', doing: 'Predicting' },
  { type: 'transaction.create', label: 'Create', doing: 'Creating' },
  { type: 'pipeline.report', label: 'Report', doing: 'Reporting' },
  { type: 'transaction.learn', label: 'Learn', doing: 'Learning' },
];

const STEP_LABELS = new Map([
  ['mailbox.changed', 'Mailbox'],
  ['mailbox.backfill', 'Import'],
  ...MESSAGE_STEPS.map((step): [string, string] => [step.type, step.label]),
]);

export function jobLabel(type: string): string {
  return STEP_LABELS.get(type) ?? type;
}

/** Which way a job is going, as one word the UI can style. */
export type JobState = 'done' | 'running' | 'queued' | 'retrying' | 'failed' | 'cancelled';

export function jobState(job: { status: JobStatus; attempt: number }): JobState {
  switch (job.status) {
    case 'succeeded':
      return 'done';
    case 'running':
      return 'running';
    case 'queued':
      return job.attempt > 0 ? 'retrying' : 'queued';
    case 'dead':
      return 'failed';
    case 'cancelled':
      return 'cancelled';
  }
}

/** A straight line of jobs, plus whatever it forks into. */
export interface JobRow {
  jobs: JobSummary[];
  children: JobRow[];
}

/**
 * Builds rows from a tree. A job with one child stays on its parent's row, so
 * a chain never indents. A new row starts only at a fork, or at a message
 * under a mailbox job, which always gets a row of its own.
 */
export function buildRows(tree: JobTree): JobRow | null {
  const root = tree.jobs.find((job) => job.id === tree.rootId) ?? tree.jobs[0];
  if (!root) return null;

  const byParent = new Map<string, JobSummary[]>();
  for (const job of tree.jobs) {
    if (!job.parentId) continue;
    byParent.set(job.parentId, [...(byParent.get(job.parentId) ?? []), job]);
  }

  const buildRow = (start: JobSummary): JobRow => {
    const jobs = [start];
    let current = start;
    for (;;) {
      const next = byParent.get(current.id) ?? [];
      if (next.length !== 1 || next[0].type === 'message.received') {
        return { jobs, children: next.map(buildRow) };
      }
      jobs.push(next[0]);
      current = next[0];
    }
  };
  return buildRow(root);
}

export function rowJobs(row: JobRow): JobSummary[] {
  return [...row.jobs, ...row.children.flatMap(rowJobs)];
}

/** The state of a row: a failure wins, then work in progress. */
export function rowState(row: JobRow): JobState {
  const states = rowJobs(row).map(jobState);
  for (const state of ['failed', 'retrying', 'running', 'queued', 'cancelled'] as const) {
    if (states.includes(state)) return state;
  }
  return 'done';
}

/** How a message ended, from its report job's label. */
export function rowOutcome(row: JobRow): string | null {
  return row.jobs.find((job) => job.type === 'pipeline.report')?.label ?? null;
}

/** Outcomes where the message ended without a transaction it should have had. */
const STOPPED_OUTCOMES = ['unverified', 'no_prediction'];

export type StepState = 'done' | 'running' | 'queued' | 'retrying' | 'failed' | 'stopped' | 'skipped' | 'pending';

export interface MessageStep {
  label: string;
  state: StepState;
  job?: JobSummary;
}

/**
 * Every visible step of a message. A step before the last one that ran but with
 * no job of its own was skipped (a memory hit skips Predict). The report isn't
 * shown: it's bookkeeping, not something that happens to the email.
 */
export function messageSteps(row: JobRow): MessageStep[] {
  const known = new Map(row.jobs.map((job) => [job.type, job]));
  // Learn only shows once it has a job: a message that stopped early never learns.
  const steps = MESSAGE_STEPS.filter(
    (step) => step.type !== 'pipeline.report' && (step.type !== 'transaction.learn' || known.has(step.type)),
  );
  const lastRan = steps.reduce((last, step, index) => (known.has(step.type) ? index : last), -1);
  const stopped = STOPPED_OUTCOMES.includes(rowOutcome(row) ?? '');

  return steps.map(({ type, label }, index) => {
    const job = known.get(type);
    if (!job) return { label, state: index < lastRan ? 'skipped' : 'pending' };

    const state = jobState(job);
    // The job ran, but found the transaction already there.
    if (state === 'done' && type === 'transaction.create' && rowOutcome(row) === 'transaction_duplicate') {
      return { label, state: 'skipped', job };
    }
    if (state === 'cancelled' || (state === 'done' && stopped && index === lastRan)) {
      return { label, state: 'stopped', job };
    }
    return { label, state, job };
  });
}

/** The step a message ended on, for "Stopped at Verify". */
export function lastStepLabel(row: JobRow): string {
  const last = [...row.jobs].reverse().find((job) => job.type !== 'pipeline.report');
  return last ? jobLabel(last.type) : '';
}

/** The job that explains a row: what's failing, else how it ended. */
export function focusJob(row: JobRow): JobSummary {
  return (
    row.jobs.find((job) => ['failed', 'retrying'].includes(jobState(job))) ??
    row.jobs.find((job) => job.type === 'pipeline.report') ??
    row.jobs[row.jobs.length - 1]
  );
}

export type EmailGroup = 'attention' | 'active' | 'created' | 'skipped' | 'cancelled';

export function emailGroup(row: JobRow): EmailGroup {
  const state = rowState(row);
  if (state === 'failed' || state === 'retrying') return 'attention';
  if (state === 'running' || state === 'queued') return 'active';
  if (state === 'cancelled') return 'cancelled';

  const outcome = rowOutcome(row);
  if (outcome === 'transaction_created') return 'created';
  return STOPPED_OUTCOMES.includes(outcome ?? '') ? 'attention' : 'skipped';
}

/** One top-level job: a mailbox sync with its emails, or a job on its own. */
export interface JobRun {
  id: string;
  root: JobSummary;
  kind: 'mailbox' | 'message' | 'other';
  /** The rows a person cares about: emails, plus the sync itself if it broke. */
  rows: JobRow[];
  emailCount: number;
  finished: number;
  /** Still fetching, so the number of emails isn't known yet. */
  reading: boolean;
  active: boolean;
  /** What most of the work is doing now, like "Checking". */
  doing: string | null;
}

/** Jobs that fan out into one message.received per email. */
const MAILBOX_JOBS = ['mailbox.changed', 'mailbox.backfill'];

export function isMailboxJob(type: string): boolean {
  return MAILBOX_JOBS.includes(type);
}

const ACTIVE_STATES: JobState[] = ['running', 'queued', 'retrying'];

function currentStep(jobs: JobSummary[]): string | null {
  const running = jobs.find((job) => jobState(job) === 'running');
  return MESSAGE_STEPS.find((step) => step.type === running?.type)?.doing ?? null;
}

export function buildRun(tree: JobTree): JobRun | null {
  const row = buildRows(tree);
  if (!row) return null;

  const root = row.jobs[0];
  const jobs = rowJobs(row);
  const active = jobs.some((job) => ACTIVE_STATES.includes(jobState(job)));
  const base = { id: tree.rootId, root, active, doing: currentStep(jobs) };

  if (!isMailboxJob(root.type)) {
    return {
      ...base,
      kind: root.type === 'message.received' ? 'message' : 'other',
      rows: [row],
      emailCount: root.type === 'message.received' ? 1 : 0,
      finished: active ? 0 : 1,
      reading: false,
    };
  }

  const rootState = jobState(root);
  const brokenSync = rootState === 'failed' || rootState === 'retrying';
  return {
    ...base,
    kind: 'mailbox',
    rows: [...(brokenSync ? [{ jobs: row.jobs, children: [] }] : []), ...row.children],
    emailCount: row.children.length,
    finished: row.children.filter((child) => {
      const state = rowState(child);
      return state !== 'running' && state !== 'queued' && state !== 'retrying';
    }).length,
    reading: rootState === 'running' || rootState === 'queued',
  };
}

export interface JobAttempt {
  startedAt: string;
  actor: string;
  /** Null while it runs, or if it ended without saving a result. */
  end: JobEvent | null;
  durationMs: number | null;
}

/**
 * Groups a job's history into attempts. Attempts are split on `claimed` rather
 * than the attempt number, because a manual retry starts the numbers again.
 */
export function buildAttempts(events: JobEvent[]): JobAttempt[] {
  const attempts: JobAttempt[] = [];
  for (const event of events) {
    if (event.type === 'claimed') {
      attempts.push({
        startedAt: event.createdAt,
        actor: event.actor,
        end: null,
        durationMs: null,
      });
      continue;
    }
    const open = attempts.at(-1);
    if (!open || open.end) continue;
    if (event.type === 'completed' || event.type === 'retry_scheduled' || event.type === 'dead') {
      open.end = event;
      open.durationMs = event.durationMs ?? new Date(event.createdAt).getTime() - new Date(open.startedAt).getTime();
    }
  }
  return attempts;
}
