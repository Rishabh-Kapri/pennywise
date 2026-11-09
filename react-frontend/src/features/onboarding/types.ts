/** Wizard steps in order; mirrors model.OnboardingSteps on the API. */
export const ONBOARDING_STEPS = ['welcome', 'accounts', 'bills', 'ai', 'email', 'assign', 'done'] as const;

export type OnboardingStep = (typeof ONBOARDING_STEPS)[number];

/** budgets.onboarding. An empty step means the wizard hasn't started. */
export interface BudgetOnboarding {
  step?: OnboardingStep | '';
  completedAt?: string;
  skippedSteps?: OnboardingStep[];
  checklistDismissed?: boolean;
  /** When this budget's onboarding import started. It's only offered once. */
  emailsImportedAt?: string;
}

export interface UpdateOnboardingPayload {
  step?: OnboardingStep;
  skippedSteps?: OnboardingStep[];
  checklistDismissed?: boolean;
  emailsImported?: boolean;
}

export type AIProviderName = 'openai' | 'anthropic' | 'openrouter' | 'ollama';

/** A saved key (user_llm_providers row). The key never comes back, only whether one is saved. */
export interface AIProvider {
  id: string;
  provider: AIProviderName;
  hasApiKey: boolean;
  apiKeyHint?: string;
  baseUrl?: string;
  enabled: boolean;
}

/** PUT body. Leave apiKey out to keep the saved key. */
export interface UpsertAIProviderPayload {
  apiKey?: string;
  baseUrl?: string;
  enabled?: boolean;
}

/**
 * chat: a conversational LLM (reading emails, predicting categories, the assistant).
 * decision: a small classification model called through a decisions API at an endpoint the user gives.
 */
export type AIModelKind = 'chat' | 'decision' | 'embedding';

/** A model the user runs with one of their keys (user_llm_models row). */
export interface AIModel {
  id: string;
  providerId: string;
  provider: AIProviderName;
  kind: AIModelKind;
  slug: string;
  /** Decision models only. */
  endpoint?: string;
  isDefault: boolean;
  /** A disabled model is kept but never run. */
  enabled: boolean;
  /** Orders the models of a kind, after the default. Lower runs first. */
  priority: number;
  lastVerifiedAt?: string;
  lastError?: string;
}

/** POST /ai-models. The server tests the model before saving it. */
export interface CreateAIModelPayload {
  provider: AIProviderName;
  kind: AIModelKind;
  slug: string;
  endpoint?: string;
  makeDefault?: boolean;
}

/** PUT /ai-models/:id. Tested again before it's saved. */
export interface UpdateAIModelPayload {
  slug: string;
  endpoint?: string;
}

export interface AIModelTestResult {
  ok: boolean;
  model?: string;
  error?: string;
  latencyMs: number;
}

/** Models whose key is enabled: the ones the server will actually use. */
export function usableModels(providers: AIProvider[], models: AIModel[], kind?: AIModelKind): AIModel[] {
  const enabled = new Set(providers.filter((p) => p.enabled).map((p) => p.id));
  return models.filter((m) => m.enabled && enabled.has(m.providerId) && (!kind || m.kind === kind));
}
