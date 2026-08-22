import type { LLMCall, PipelineRun, PipelineRunEvent, PipelineStep } from './types';

export const STEPPER: { key: PipelineStep; label: string }[] = [
  { key: 'fetch_emails', label: 'Fetch' },
  { key: 'parse', label: 'Extract' },
  { key: 'predict', label: 'Predict' },
  { key: 'create_transactions', label: 'Create' }
];

const STEP_ORDER: PipelineStep[] = [
  'fetch_user',
  'fetch_emails',
  'parse',
  'predict',
  'create_transactions',
  'done'
];

export type StepState = 'done' | 'active' | 'waiting' | 'error' | 'pending';

export function stepState(run: PipelineRun, step: PipelineStep): StepState {
  if (run.status === 'completed') {
    return 'done';
  }
  // fetch_user is preparation for the fetch step, show it under "Fetch"
  const currentStep = run.currentStep === 'fetch_user' ? 'fetch_emails' : run.currentStep;
  const currentIndex = STEP_ORDER.indexOf(currentStep);
  const stepIndex = STEP_ORDER.indexOf(step);

  if (stepIndex < currentIndex) return 'done';
  if (stepIndex > currentIndex) return 'pending';
  if (run.status === 'failed') return 'error';
  if (run.status === 'waiting_retry') return 'waiting';
  return 'active';
}

export const STATUS_LABELS: Record<PipelineRun['status'], string> = {
  running: 'Running',
  waiting_retry: 'Needs retry',
  completed: 'Completed',
  failed: 'Failed'
};

export function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const diffMinutes = Math.floor((Date.now() - date.getTime()) / 60_000);

  if (diffMinutes < 1) return 'just now';
  if (diffMinutes < 60) return `${diffMinutes}m ago`;
  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours}h ago`;

  return date.toLocaleString(undefined, {
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit'
  });
}

export function detailString(event: PipelineRunEvent | undefined, key: string): string {
  const value = event?.detail?.[key];
  if (value === undefined || value === null) return '';
  return String(value);
}

export function detailNumber(event: PipelineRunEvent | undefined, key: string): number | null {
  const value = event?.detail?.[key];
  return typeof value === 'number' ? value : null;
}

export interface EmailGroup {
  messageId: string;
  fetch?: PipelineRunEvent;
  parse?: PipelineRunEvent;
  predict?: PipelineRunEvent;
}

export function groupEventsByEmail(events: PipelineRunEvent[]): EmailGroup[] {
  const groups = new Map<string, EmailGroup>();
  for (const event of events) {
    if (!event.messageId) continue;
    let group = groups.get(event.messageId);
    if (!group) {
      group = { messageId: event.messageId };
      groups.set(event.messageId, group);
    }
    if (event.step === 'fetch_emails') group.fetch = event;
    if (event.step === 'parse') group.parse = event;
    if (event.step === 'predict') group.predict = event;
  }
  return [...groups.values()];
}

/** "Bank Alerts <alerts@bank.com>" → "Bank Alerts"; bare addresses pass through. */
export function formatSender(from: string): string {
  const match = from.match(/^\s*"?([^"<]*?)"?\s*<(.+)>\s*$/);
  if (!match) return from.trim();
  return match[1].trim() || match[2].trim();
}

export function senderAddress(from: string): string {
  const match = from.match(/<(.+)>/);
  return match ? match[1].trim() : '';
}

/** 900 → "900", 1_250 → "1.3k". */
export function formatTokens(count: number): string {
  if (count < 1000) return String(count);
  return `${(count / 1000).toFixed(1)}k`;
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

/** The model calls recorded on one email's timeline event. */
export function eventCalls(event: PipelineRunEvent | undefined): LLMCall[] {
  const value = event?.detail?.llmCalls;
  return Array.isArray(value) ? (value as LLMCall[]) : [];
}

/** One-line "model · 1.2k in / 96 out · 4.1s" summary for a step's calls. */
export function callSummary(calls: LLMCall[]): string {
  if (calls.length === 0) return '';
  const models = [...new Set(calls.filter((call) => !call.failed).map((call) => call.model))];
  const inputTokens = calls.reduce((total, call) => total + (call.inputTokens ?? 0), 0);
  const outputTokens = calls.reduce((total, call) => total + (call.outputTokens ?? 0), 0);
  const durationMs = calls.reduce((total, call) => total + (call.durationMs ?? 0), 0);

  const parts = [models.join(', ') || 'no model'];
  if (inputTokens || outputTokens) {
    parts.push(`${formatTokens(inputTokens)} in / ${formatTokens(outputTokens)} out`);
  }
  if (durationMs) parts.push(formatDuration(durationMs));
  return parts.join(' · ');
}

/** The "3 emails · 1 skipped · 2 transactions" line under a run's header. */
export function runCounts(run: PipelineRun): string {
  const counts: string[] = [];
  if (run.emailsFetched > 0) {
    counts.push(`${run.emailsFetched} email${run.emailsFetched === 1 ? '' : 's'}`);
  }
  if (run.emailsSkipped > 0) {
    counts.push(`${run.emailsSkipped} skipped`);
  }
  if (run.transactionsCreated > 0) {
    counts.push(`${run.transactionsCreated} transaction${run.transactionsCreated === 1 ? '' : 's'}`);
  }
  if (run.status === 'completed' && run.emailsFetched === 0) {
    counts.push('no transaction emails');
  }
  return counts.join(' · ');
}
