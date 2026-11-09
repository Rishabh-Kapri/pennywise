import type { AIProviderName } from './types';

export interface ProviderInfo {
  label: string;
  /** A sensible first chat model for this provider. */
  defaultChatModel: string;
  /** Set only for providers Pennywise can embed with. Must return 1024 dimensions. */
  defaultEmbeddingModel?: string;
  keyPlaceholder: string;
  keyUrl?: string;
  /** Ollama: the user's own server, key optional. */
  selfHosted?: boolean;
}

export const AI_PROVIDERS: Record<AIProviderName, ProviderInfo> = {
  openai: {
    label: 'OpenAI',
    defaultChatModel: 'gpt-4o-mini',
    keyPlaceholder: 'sk-…',
    keyUrl: 'https://platform.openai.com/api-keys',
  },
  anthropic: {
    label: 'Anthropic',
    defaultChatModel: 'claude-haiku-4-5',
    keyPlaceholder: 'sk-ant-…',
    keyUrl: 'https://console.anthropic.com/settings/keys',
  },
  openrouter: {
    label: 'OpenRouter',
    defaultChatModel: 'google/gemini-2.5-flash',
    defaultEmbeddingModel: 'baai/bge-m3',
    keyPlaceholder: 'sk-or-…',
    keyUrl: 'https://openrouter.ai/keys',
  },
  ollama: {
    label: 'Ollama (self-hosted)',
    defaultChatModel: 'gemma4:12b',
    defaultEmbeddingModel: 'bge-m3',
    keyPlaceholder: 'Optional',
    selfHosted: true,
  },
};
