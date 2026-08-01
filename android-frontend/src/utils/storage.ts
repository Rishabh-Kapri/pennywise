import AsyncStorage from '@react-native-async-storage/async-storage';
import type { AuthTokens, User } from '../features/auth/types';

/**
 * Exported because background tasks (widgets, the location snap) read this key
 * directly -- they run without a Redux store, so `utils/headlessApi` goes
 * straight to storage rather than through `apiClient`.
 */
export const AUTH_STORAGE_KEY = 'pennywise_auth';

/**
 * The selected budget lives in Redux, which background tasks cannot see. It is
 * mirrored here so a headless request can send `x-budget-id` without the app
 * being open. Written by `budgetPersistenceMiddleware`.
 */
export const SELECTED_BUDGET_STORAGE_KEY = 'pennywise_selected_budget_id';

export async function loadAuthFromStorage(): Promise<{
  user: User | null;
  tokens: AuthTokens | null;
}> {
  try {
    const raw = await AsyncStorage.getItem(AUTH_STORAGE_KEY);
    if (!raw) return { user: null, tokens: null };
    const parsed = JSON.parse(raw) as { user?: User; tokens?: AuthTokens };
    if (!parsed.tokens?.refreshToken || !parsed.user) {
      await AsyncStorage.removeItem(AUTH_STORAGE_KEY);
      return { user: null, tokens: null };
    }
    return { user: parsed.user, tokens: parsed.tokens };
  } catch {
    return { user: null, tokens: null };
  }
}

export async function saveAuthToStorage(user: User, tokens: AuthTokens) {
  await AsyncStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify({ user, tokens }));
}

export async function clearAuthFromStorage() {
  // Drop the budget alongside the session: a stale budget id outliving a logout
  // would leave background tasks querying the previous account's budget.
  await AsyncStorage.multiRemove([AUTH_STORAGE_KEY, SELECTED_BUDGET_STORAGE_KEY]);
}

export async function saveSelectedBudgetId(budgetId: string): Promise<void> {
  await AsyncStorage.setItem(SELECTED_BUDGET_STORAGE_KEY, budgetId);
}

export async function loadSelectedBudgetId(): Promise<string | null> {
  try {
    return await AsyncStorage.getItem(SELECTED_BUDGET_STORAGE_KEY);
  } catch {
    return null;
  }
}
