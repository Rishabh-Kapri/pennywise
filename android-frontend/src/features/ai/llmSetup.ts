import { apiClient } from '../../utils/api';

export type AIProviderName = 'openai' | 'anthropic' | 'openrouter' | 'ollama' | 'typesafe';
export type AIModelKind = 'chat' | 'decision' | 'embedding';

/** A saved key. The key itself never comes back. */
export interface AIProvider {
  id: string;
  provider: AIProviderName;
  hasApiKey: boolean;
  apiKeyHint?: string;
  baseUrl?: string;
  enabled: boolean;
}

export interface AIModel {
  id: string;
  providerId: string;
  provider: AIProviderName;
  kind: AIModelKind;
  slug: string;
  endpoint?: string;
  isDefault: boolean;
  enabled: boolean;
  priority: number;
  lastVerifiedAt?: string;
  lastError?: string;
}

export interface AIModelTestResult {
  ok: boolean;
  model?: string;
  error?: string;
  latencyMs: number;
}

export const AI_PROVIDERS: Record<
  AIProviderName,
  {
    label: string;
    mark: string;
    defaultChatModel: string;
    defaultEmbeddingModel?: string;
    selfHosted?: boolean;
    /** Set only for decision-only providers, which fill in the model form. */
    decisionModel?: { slug: string; endpoint: string };
  }
> = {
  openai: { label: 'OpenAI', mark: 'OA', defaultChatModel: 'gpt-4o-mini' },
  anthropic: { label: 'Anthropic', mark: 'An', defaultChatModel: 'claude-haiku-4-5' },
  openrouter: {
    label: 'OpenRouter',
    mark: 'OR',
    defaultChatModel: 'google/gemini-2.5-flash',
    defaultEmbeddingModel: 'baai/bge-m3'
  },
  ollama: {
    label: 'Ollama (self-hosted)',
    mark: 'Ol',
    defaultChatModel: 'gemma4:12b',
    defaultEmbeddingModel: 'bge-m3',
    selfHosted: true
  },
  typesafe: {
    label: 'TypeSafe (Jev)',
    mark: 'TS',
    defaultChatModel: '',
    decisionModel: { slug: 'jev-latest', endpoint: 'https://api.typesafe.ai/v1/systemone' }
  }
};

export const PROVIDER_NAMES = Object.keys(AI_PROVIDERS) as AIProviderName[];

export const MODEL_KINDS: { kind: AIModelKind; title: string; description: string; noun: string; empty: string }[] = [
  {
    kind: 'chat',
    title: 'Chat models',
    description: 'Read bank emails, predict payees and categories, and run the assistant.',
    noun: 'chat model',
    empty: 'No chat models yet.'
  },
  {
    kind: 'decision',
    title: 'Decision models',
    description: 'Small, cheap classification models called through a decisions API.',
    noun: 'decision model',
    empty: 'No decision models yet.'
  },
  {
    kind: 'embedding',
    title: 'Embedding models',
    description: 'Find similar past transactions. Needs an OpenRouter or Ollama key.',
    noun: 'embedding model',
    empty: 'No embedding models yet.'
  }
];

export const EMBEDDING_SWITCH_WARNING =
  'Past transactions were matched using your current model, so switching makes those matches stop working until they are learned again.';

export function defaultSlug(kind: AIModelKind, provider: AIProviderName): string {
  if (kind === 'chat') return AI_PROVIDERS[provider].defaultChatModel;
  if (kind === 'embedding') return AI_PROVIDERS[provider].defaultEmbeddingModel ?? '';
  return AI_PROVIDERS[provider].decisionModel?.slug ?? '';
}

export function defaultEndpoint(kind: AIModelKind, provider: AIProviderName): string {
  return kind === 'decision' ? (AI_PROVIDERS[provider].decisionModel?.endpoint ?? '') : '';
}

export function servesKind(kind: AIModelKind, provider: AIProviderName): boolean {
  if (AI_PROVIDERS[provider].decisionModel) return kind === 'decision';
  return kind !== 'embedding' || !!AI_PROVIDERS[provider].defaultEmbeddingModel;
}

/** Enabled models on enabled keys: the ones the server will actually use. */
export function usableModels(providers: AIProvider[], models: AIModel[]): AIModel[] {
  const enabled = new Set(providers.filter((p) => p.enabled).map((p) => p.id));
  return models.filter((m) => m.enabled && enabled.has(m.providerId));
}

export const llmApi = {
  providers: () => apiClient.get<AIProvider[]>('ai-providers'),
  saveProvider: (provider: AIProviderName, body: { apiKey?: string; baseUrl?: string; enabled?: boolean }) =>
    apiClient.put<AIProvider>(`ai-providers/${provider}`, body),
  deleteProvider: (provider: AIProviderName) => apiClient.delete<void>(`ai-providers/${provider}`),
  models: () => apiClient.get<AIModel[]>('ai-models'),
  createModel: (body: {
    provider: AIProviderName;
    kind: AIModelKind;
    slug: string;
    endpoint?: string;
    makeDefault?: boolean;
  }) => apiClient.post<AIModel>('ai-models', body),
  updateModel: (id: string, body: { slug: string; endpoint?: string }) =>
    apiClient.put<AIModel>(`ai-models/${id}`, body),
  setDefault: (id: string) => apiClient.post<AIModel>(`ai-models/${id}/default`, {}),
  setEnabled: (id: string, enabled: boolean) => apiClient.put<AIModel>(`ai-models/${id}/enabled`, { enabled }),
  reorder: (kind: AIModelKind, ids: string[]) => apiClient.put<void>('ai-models/order', { kind, ids }),
  deleteModel: (id: string) => apiClient.delete<void>(`ai-models/${id}`),
  testModel: (id: string) => apiClient.post<AIModelTestResult>(`ai-models/${id}/test`, {})
};

export function errorText(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}
