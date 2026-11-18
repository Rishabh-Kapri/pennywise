import type { JobSummary, ModelUsage } from './types/job.types';
import { jobLabel } from './jobTree';

export interface UsageTotal {
  calls: number;
  inputTokens: number;
  outputTokens: number;
  costMicros: number;
  /** Some call reported a cost. Without one the total is unknown, not free. */
  priced: boolean;
}

export interface StepUsage extends UsageTotal {
  jobId: string;
  label: string;
  model?: string;
}

function add(total: UsageTotal, usage: ModelUsage): UsageTotal {
  return {
    calls: total.calls + usage.calls,
    inputTokens: total.inputTokens + (usage.inputTokens ?? 0),
    outputTokens: total.outputTokens + (usage.outputTokens ?? 0),
    costMicros: total.costMicros + (usage.costMicros ?? 0),
    priced: total.priced || usage.costMicros !== undefined,
  };
}

const EMPTY: UsageTotal = { calls: 0, inputTokens: 0, outputTokens: 0, costMicros: 0, priced: false };

/** What the model calls of these jobs added up to; null when none called a model. */
export function totalUsage(jobs: JobSummary[]): UsageTotal | null {
  const used = jobs.filter((job) => job.usage && job.usage.calls > 0);
  if (used.length === 0) return null;
  return used.reduce((total, job) => add(total, job.usage!), EMPTY);
}

/** One line per step that called a model, in pipeline order. */
export function stepUsage(jobs: JobSummary[]): StepUsage[] {
  return jobs
    .filter((job) => job.usage && job.usage.calls > 0)
    .map((job) => ({ ...add(EMPTY, job.usage!), jobId: job.id, label: jobLabel(job.type), model: job.usage!.model }));
}
