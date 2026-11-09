import type { AIModelKind, AIProviderName } from '@/features/onboarding/types';

/**
 * Pipeline and step IDs come from the pipeline files on the server, and each
 * pipeline has its own steps. They're stored in budget overrides, so they
 * never change.
 */
export type PipelineId = string;
export type PipelineStepId = string;

/** One way of running a step, as listed in its pipeline file. */
export interface PipelineAdapter {
  id: string;
  label: string;
  description: string;
  /** Calls one of the user's models, of kind modelKind, so a model can be picked. */
  usesModel?: boolean;
  modelKind?: AIModelKind;
  /** Listed but not built yet; can't be chosen. */
  unavailable?: boolean;
}

/** One of the user's models, as the pipeline refers to it. */
export interface PipelineModelRef {
  id: string;
  kind: AIModelKind;
  provider: AIProviderName;
  slug: string;
}

/** What a step runs with. model is set only for model adapters when the user has one of that kind. */
export interface PipelineStepChoice {
  adapter: string;
  model?: PipelineModelRef;
  /** When the model fails, try the user's other models of its kind, by priority. */
  fallback: boolean;
}

/** A budget's change to one step. A null field follows the default. */
export interface PipelineStepOverride {
  adapter: string | null;
  modelId: string | null;
  fallback: boolean | null;
  updatedAt: string;
}

export interface PipelineStep {
  id: PipelineStepId;
  label: string;
  description: string;
  adapters: PipelineAdapter[];
  defaultAdapter: string;
  /** The system default for falling back to the user's other models. */
  fallback?: boolean;
  /** null when the budget hasn't changed this step. */
  override: PipelineStepOverride | null;
  /** The default with the override applied: what the next message will use. */
  effective: PipelineStepChoice;
  /**
   * Set when the override can't be used (its adapter was retired, its model
   * was removed) and the step fell back, or when the step needs a kind of
   * model the user hasn't set up.
   */
  warning?: string;
}

/** One system pipeline (messages, statements, …) with this budget's overrides applied. */
export interface Pipeline {
  id: PipelineId;
  label: string;
  description: string;
  version: number;
  steps: PipelineStep[];
}

/** GET /pipelines: every system pipeline for this budget. */
export interface PipelineCatalog {
  /** The owner's default model of each kind they've set up, shared by every pipeline. */
  defaults: Partial<Record<AIModelKind, PipelineModelRef>>;
  pipelines: Pipeline[];
}

/** PUT /pipelines/:pipeline/steps/:step body. Send null to follow the default. */
export interface UpdatePipelineStepPayload {
  adapter: string | null;
  modelId: string | null;
  fallback: boolean | null;
}
