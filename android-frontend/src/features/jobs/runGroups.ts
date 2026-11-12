// Keep in step with react-frontend/src/features/jobs/runGroups.ts.
import type { JobRun } from './jobTree';

/** A row on the Jobs list. `startedAt` is set when it stands for several runs. */
export interface ListedRun extends JobRun {
  startedAt?: string;
}

/** One day's rows on the Jobs list, newest first. */
export interface JobDay {
  key: string;
  /** Any time on this day. */
  date: string;
  runs: ListedRun[];
}

function dayOf(run: JobRun): string {
  return new Date(run.root.createdAt).toDateString();
}

/** The source a run is folded under, or null for a run that keeps its own row. */
function sourceOf(run: JobRun): string | null {
  // A message with no sync above it is its own job (SMS, API).
  if (run.kind === 'message') return run.root.source ? `message:${run.root.source}` : null;
  if (run.root.type !== 'mailbox.changed') return null;
  // A sync that's running or broke shows its own progress or error, and
  // joins the day's row once it has finished.
  const finished = !run.active && run.root.status === 'succeeded' && run.rows.length > 0;
  return finished ? `mailbox:${run.root.label}` : null;
}

function merge(newer: ListedRun, older: JobRun): ListedRun {
  return {
    ...newer,
    // The oldest run's ID, so a new one joining the group doesn't remount the row.
    id: older.id,
    rows: [...newer.rows, ...older.rows],
    emailCount: newer.emailCount + older.emailCount,
    finished: newer.finished + older.finished,
    active: newer.active || older.active,
    doing: newer.doing ?? older.doing,
    startedAt: older.root.createdAt,
  };
}

/**
 * Folds runs into one row per source and day. `runs` is newest first; a row
 * sits where its newest run was.
 */
export function mergeBySource(runs: JobRun[]): ListedRun[] {
  const merged: ListedRun[] = [];
  const rowOf = new Map<string, number>();
  for (const run of runs) {
    const source = sourceOf(run);
    const index = source === null ? undefined : rowOf.get(`${source}:${dayOf(run)}`);
    if (index !== undefined) {
      merged[index] = merge(merged[index], run);
      continue;
    }
    if (source !== null) rowOf.set(`${source}:${dayOf(run)}`, merged.length);
    merged.push(run);
  }
  return merged;
}

/** Splits rows into days, newest day first. `runs` is newest first. */
export function splitByDay(runs: ListedRun[]): JobDay[] {
  const days = new Map<string, JobDay>();
  for (const run of runs) {
    const key = dayOf(run);
    const day = days.get(key);
    if (day) day.runs.push(run);
    else days.set(key, { key, date: run.root.createdAt, runs: [run] });
  }
  return [...days.values()];
}
