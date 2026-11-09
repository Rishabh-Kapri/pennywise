import { apiClient } from '@/utils';
import type {
  AIModel,
  AIModelKind,
  AIModelTestResult,
  AIProvider,
  AIProviderName,
  BudgetOnboarding,
  CreateAIModelPayload,
  UpdateAIModelPayload,
  UpdateOnboardingPayload,
  UpsertAIProviderPayload,
} from './types';
import { usableModels } from './types';

export const updateOnboarding = (payload: UpdateOnboardingPayload) =>
  apiClient.patch<BudgetOnboarding, UpdateOnboardingPayload>('onboarding', payload);

/** The user's saved keys, one per provider. */
export const fetchAIProviders = () => apiClient.get<AIProvider[]>('ai-providers');

export const saveAIProvider = (provider: AIProviderName, payload: UpsertAIProviderPayload) =>
  apiClient.put<AIProvider, UpsertAIProviderPayload>(`ai-providers/${provider}`, payload);

export const deleteAIProvider = (provider: AIProviderName) => apiClient.delete<void>(`ai-providers/${provider}`);

export const fetchAIModels = () => apiClient.get<AIModel[]>('ai-models');

/** Tests the model (and its endpoint, for decision models) before saving; a failed test is a 422. */
export const createAIModel = (payload: CreateAIModelPayload) =>
  apiClient.post<AIModel, CreateAIModelPayload>('ai-models', payload);

export const updateAIModel = (id: string, payload: UpdateAIModelPayload) =>
  apiClient.put<AIModel, UpdateAIModelPayload>(`ai-models/${id}`, payload);

export const setDefaultAIModel = (id: string) =>
  apiClient.post<AIModel, Record<string, never>>(`ai-models/${id}/default`, {});

export const setAIModelEnabled = (id: string, enabled: boolean) =>
  apiClient.put<AIModel, { enabled: boolean }>(`ai-models/${id}/enabled`, { enabled });

/** ids must list every model of the kind, in the order they should run. */
export const reorderAIModels = (kind: AIModelKind, ids: string[]) =>
  apiClient.put<void, { kind: AIModelKind; ids: string[] }>('ai-models/order', { kind, ids });

export const deleteAIModel = (id: string) => apiClient.delete<void>(`ai-models/${id}`);

export const testAIModel = (id: string) =>
  apiClient.post<AIModelTestResult, Record<string, never>>(`ai-models/${id}/test`, {});

/** Whether the user has a chat model on an enabled key: what email import and the assistant need. */
export const fetchHasChatModel = () =>
  Promise.all([fetchAIProviders(), fetchAIModels()]).then(
    ([providers, models]) => usableModels(providers ?? [], models ?? [], 'chat').length > 0,
  );
