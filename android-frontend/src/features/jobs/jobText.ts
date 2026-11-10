import { focusJob, jobLabel, jobState, lastStepLabel, rowOutcome, rowState, type JobRow, type JobState } from './jobTree';

export const STATE_LABELS: Record<JobState, string> = {
  done: 'Done',
  running: 'Running',
  queued: 'Queued',
  retrying: 'Retrying',
  failed: 'Failed',
  cancelled: 'Cancelled'
};

const OUTCOME_LABELS: Record<string, string> = {
  not_transaction: 'Not a transaction',
  unverified: 'Not verified',
  no_prediction: 'No category found',
  needs_review: 'Needs review',
  transaction_created: 'Transaction created',
  transaction_duplicate: 'Already added'
};

const OUTCOME_REASONS: Record<string, string> = {
  not_transaction: "The check decided this message isn't a transaction.",
  unverified: "Marked as a transaction, but it couldn't be verified.",
  no_prediction: "Couldn't work out a payee and category for it.",
  needs_review: 'A step was not sure about this message. Review it on the web app.',
  transaction_created: 'A transaction was created from this message.',
  transaction_duplicate: 'This matches a transaction that already exists, so nothing new was added.'
};

export function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`;
}

export function rowTitle(row: JobRow): string {
  const first = row.jobs[0];
  if (first.type === 'mailbox.backfill') return 'Reading the chosen emails';
  if (first.type === 'mailbox.changed') return 'Reading the mailbox';
  return first.label || jobLabel(first.type);
}

export function rowStatus(row: JobRow): string {
  const state = rowState(row);
  if (state !== 'done') return STATE_LABELS[state];
  const outcome = rowOutcome(row);
  return outcome ? (OUTCOME_LABELS[outcome] ?? outcome) : STATE_LABELS.done;
}

export function rowReason(row: JobRow, reason?: string): string {
  const job = focusJob(row);
  const state = jobState(job);
  const step = lastStepLabel(row);
  if (state === 'failed') return `${step} failed after ${plural(job.attempt, 'attempt', 'attempts')}.`;
  if (state === 'retrying') return `${step} failed. Trying again, attempt ${job.attempt + 1} of ${job.maxAttempts}.`;
  const outcome = rowOutcome(row) ?? '';
  return reason || OUTCOME_REASONS[outcome] || STATE_LABELS[rowState(row)];
}

export function formatTime(date: string): string {
  return new Date(date).toLocaleString(undefined, { day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit' });
}

export function formatAgo(date: string): string {
  const minutes = Math.floor((Date.now() - new Date(date).getTime()) / 60_000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  return hours < 24 ? `${hours}h ago` : `${Math.floor(hours / 24)}d ago`;
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60_000)}m ${Math.round((ms % 60_000) / 1000)}s`;
}
