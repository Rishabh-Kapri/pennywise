import AsyncStorage from '@react-native-async-storage/async-storage';
import type { AuthTokens, User } from '../features/auth/types';

const AUTH_STORAGE_KEY = 'pennywise_auth';

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
  await AsyncStorage.removeItem(AUTH_STORAGE_KEY);
}
