import type { AIModelKind, AIProviderName } from '../ai/llmSetup';

export type JobStatus = 'queued' | 'running' | 'succeeded' | 'cancelled' | 'dead';

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

export interface JobFailure {
  code: string;
  message: string;
  count: number;
  lastSeen: string;
}

export interface JobList {
  /** False when the API has no job store. */
  enabled: boolean;
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
  payload?: unknown;
  result?: unknown;
  events: JobEvent[];
}

/** The result a message pipeline step stores. */
export interface MessageStepResult {
  output?: unknown;
  nextJobType?: string;
  outcome?: string;
}

export interface PipelineAdapter {
  id: string;
  label: string;
  description: string;
  usesModel?: boolean;
  modelKind?: AIModelKind;
  unavailable?: boolean;
}

export interface PipelineModelRef {
  id: string;
  kind: AIModelKind;
  provider: AIProviderName;
  slug: string;
}

export interface PipelineStepChoice {
  adapter: string;
  model?: PipelineModelRef;
  fallback: boolean;
}

export interface PipelineStep {
  id: string;
  label: string;
  description: string;
  adapters: PipelineAdapter[];
  defaultAdapter: string;
  fallback?: boolean;
  override: { adapter: string | null; modelId: string | null; fallback: boolean | null; updatedAt: string } | null;
  effective: PipelineStepChoice;
  warning?: string;
}

export interface Pipeline {
  id: string;
  label: string;
  description: string;
  version: number;
  steps: PipelineStep[];
}

export interface PipelineCatalog {
  defaults: Partial<Record<AIModelKind, PipelineModelRef>>;
  pipelines: Pipeline[];
}
