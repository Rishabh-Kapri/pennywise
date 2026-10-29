import { apiClient } from '@/utils';
import type {
  AIProvider,
  AIProviderName,
  AIProviderTestResult,
  BudgetOnboarding,
  EmailBackfillResponse,
  UpdateOnboardingPayload,
  UpsertAIProviderPayload,
} from './types';

export const updateOnboarding = (payload: UpdateOnboardingPayload) =>
  apiClient.patch<BudgetOnboarding, UpdateOnboardingPayload>('onboarding', payload);

/** Runs the email pipeline over recent bank alerts; transactions arrive over the websocket. */
export const startEmailBackfill = (days = 14) =>
  apiClient.post<EmailBackfillResponse, { days: number }>('onboarding/email-backfill', { days });

export const fetchAIProviders = () => apiClient.get<AIProvider[]>('ai-providers');

export const saveAIProvider = (provider: AIProviderName, payload: UpsertAIProviderPayload) =>
  apiClient.put<AIProvider, UpsertAIProviderPayload>(`ai-providers/${provider}`, payload);

export const deleteAIProvider = (provider: AIProviderName) => apiClient.delete<void>(`ai-providers/${provider}`);

export const testAIProvider = (provider: AIProviderName) =>
  apiClient.post<AIProviderTestResult, Record<string, never>>(`ai-providers/${provider}/test`, {});
