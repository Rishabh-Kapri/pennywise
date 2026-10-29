/** Wizard steps in order; mirrors model.OnboardingSteps on the API. */
export const ONBOARDING_STEPS = ['welcome', 'accounts', 'bills', 'ai', 'email', 'assign', 'done'] as const;

export type OnboardingStep = (typeof ONBOARDING_STEPS)[number];

/** budgets.onboarding. An empty step means the wizard hasn't started. */
export interface BudgetOnboarding {
  step?: OnboardingStep | '';
  completedAt?: string;
  skippedSteps?: OnboardingStep[];
  checklistDismissed?: boolean;
}

export interface UpdateOnboardingPayload {
  step?: OnboardingStep;
  skippedSteps?: OnboardingStep[];
  checklistDismissed?: boolean;
}

export interface EmailBackfillResponse {
  emailCount: number;
  workflowId?: string;
}

export type AIProviderName = 'openai' | 'anthropic' | 'openrouter' | 'ollama';

/** A user_llm_providers row. The key never comes back, only whether one is saved. */
export interface AIProvider {
  id: string;
  provider: AIProviderName;
  hasApiKey: boolean;
  apiKeyHint?: string;
  baseUrl?: string;
  model: string;
  priority: number;
  enabled: boolean;
  lastVerifiedAt?: string;
  lastError?: string;
}

/** PUT body. Leave apiKey out to keep the saved key. */
export interface UpsertAIProviderPayload {
  apiKey?: string;
  baseUrl?: string;
  model: string;
  priority?: number;
  enabled?: boolean;
}

export interface AIProviderTestResult {
  ok: boolean;
  model?: string;
  error?: string;
  latencyMs: number;
}
