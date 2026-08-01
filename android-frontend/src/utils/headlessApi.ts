import AsyncStorage from '@react-native-async-storage/async-storage';
import { config } from '../config/env';
import { AUTH_STORAGE_KEY, loadSelectedBudgetId } from './storage';

/**
 * API client for code that runs outside the React tree: widget task handlers,
 * the location-snap notification task, anything Android starts headlessly.
 *
 * `utils/api.ts` cannot be used there. It reads tokens and the selected budget
 * out of the Redux store via `setGetState`, which is only ever wired up by
 * `app/store.ts` when the app itself boots. In a headless context `getState`
 * is null, so every request goes out unauthenticated, 401s, fails to refresh,
 * and lands in `handleSessionExpired()` -- which wipes the stored session and
 * signs the user out of the app. A background refresh must never be able to do
 * that, which is why this module is deliberately separate rather than a flag
 * on the existing client.
 */

/** Refresh this far before nominal expiry, to absorb clock skew and latency. */
const REFRESH_SKEW_MS = 60 * 1000;

type StoredTokens = {
  accessToken: string;
  refreshToken: string;
  expiresAt: number;
};

type StoredAuth = {
  user?: unknown;
  tokens?: StoredTokens;
};

/** Thrown when there is no usable session. Callers should render a signed-out state. */
export class NotAuthenticatedError extends Error {
  constructor(message = 'No stored session') {
    super(message);
    this.name = 'NotAuthenticatedError';
  }
}

async function readStoredAuth(): Promise<StoredAuth | null> {
  try {
    const raw = await AsyncStorage.getItem(AUTH_STORAGE_KEY);
    return raw ? (JSON.parse(raw) as StoredAuth) : null;
  } catch {
    return null;
  }
}

/**
 * Persist a rotated access token. Read-modify-write rather than overwrite so a
 * concurrent write from the app (which stores `user` alongside `tokens`) is not
 * clobbered by a background refresh.
 */
async function persistAccessToken(accessToken: string, expiresInSeconds: number): Promise<void> {
  const stored = await readStoredAuth();
  if (!stored?.tokens) return;
  stored.tokens.accessToken = accessToken;
  stored.tokens.expiresAt = Date.now() + expiresInSeconds * 1000;
  await AsyncStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(stored));
}

/**
 * Exchange the refresh token for a fresh access token.
 *
 * The API does not rotate refresh tokens (`authService.RefreshToken` returns
 * only an access token), so two contexts refreshing at once cannot invalidate
 * each other and no cross-process lock is needed.
 */
async function refreshAccessToken(refreshToken: string): Promise<string | null> {
  try {
    const res = await fetch(`${config.apiBaseUrl}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refreshToken })
    });
    if (!res.ok) return null;
    const data = (await res.json()) as { accessToken?: string; expiresIn?: number };
    if (!data.accessToken) return null;
    await persistAccessToken(data.accessToken, data.expiresIn ?? 900);
    return data.accessToken;
  } catch {
    return null;
  }
}

/**
 * A usable access token, refreshed up front when the stored one is at or near
 * expiry. Access tokens last 15 minutes and widget wakes are at least 30 apart,
 * so the stored token is almost always stale by the time a background task
 * runs; refreshing reactively on a 401 would waste a round trip nearly every
 * time.
 */
async function getAccessToken(): Promise<string> {
  const stored = await readStoredAuth();
  if (!stored?.tokens?.refreshToken) throw new NotAuthenticatedError();

  const { accessToken, refreshToken, expiresAt } = stored.tokens;
  const stale = !accessToken || !expiresAt || Date.now() >= expiresAt - REFRESH_SKEW_MS;
  if (!stale) return accessToken;

  const refreshed = await refreshAccessToken(refreshToken);
  if (refreshed) return refreshed;

  // Refresh failed: fall back to the stored token rather than giving up. It may
  // still be valid if `expiresAt` is wrong or the refresh call merely timed out,
  // and a 401 below is recoverable. Notably we do NOT clear the session here.
  if (accessToken) return accessToken;
  throw new NotAuthenticatedError('Token refresh failed');
}

async function parse<T>(res: Response): Promise<T> {
  const text = await res.text();
  const data = text ? (JSON.parse(text) as unknown) : {};
  if (!res.ok) {
    const message = (data as { error?: string })?.error ?? res.statusText;
    throw new Error(String(message));
  }
  return data as T;
}

type RequestOptions = {
  /** Overrides the persisted budget, for callers that already know it (e.g. a push payload). */
  budgetId?: string;
  body?: unknown;
};

async function request<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
  const budgetId = options.budgetId ?? (await loadSelectedBudgetId());

  const send = (accessToken: string) => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${accessToken}`
    };
    // Mirrors apiClient: budget endpoints are not budget-scoped.
    if (budgetId && !path.includes('budgets')) headers['x-budget-id'] = budgetId;

    return fetch(`${config.apiBaseUrl}/${path}`, {
      method,
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body)
    });
  };

  const token = await getAccessToken();
  let res = await send(token);

  // Belt and braces: the proactive refresh above handles the usual case, but a
  // server-side token version bump or clock skew can still produce a 401.
  if (res.status === 401) {
    const stored = await readStoredAuth();
    const refreshToken = stored?.tokens?.refreshToken;
    if (!refreshToken) throw new NotAuthenticatedError();

    const refreshed = await refreshAccessToken(refreshToken);
    if (!refreshed) throw new NotAuthenticatedError('Session expired');
    res = await send(refreshed);
  }

  return parse<T>(res);
}

export const headlessApi = {
  get: <T>(path: string, options?: Omit<RequestOptions, 'body'>) => request<T>('GET', path, options),
  post: <T>(path: string, body?: unknown, options?: Omit<RequestOptions, 'body'>) =>
    request<T>('POST', path, { ...options, body }),
  patch: <T>(path: string, body?: unknown, options?: Omit<RequestOptions, 'body'>) =>
    request<T>('PATCH', path, { ...options, body }),
  delete: <T>(path: string, options?: Omit<RequestOptions, 'body'>) => request<T>('DELETE', path, options)
};

/** True when a session exists in storage. Cheap check for rendering signed-out states. */
export async function hasStoredSession(): Promise<boolean> {
  const stored = await readStoredAuth();
  return Boolean(stored?.tokens?.refreshToken);
}
