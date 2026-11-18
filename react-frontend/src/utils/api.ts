import { config } from '@/config/env';

/** A response from the server, as opposed to not reaching it at all. */
export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export interface RefreshedToken {
  accessToken: string;
  expiresIn: number;
}

/**
 * What the client needs from the signed-in session. The store provides it
 * (`app/store.ts`), so this module depends on nothing above it.
 */
export interface ApiSession {
  accessToken(): string | undefined;
  refreshToken(): string | undefined;
  budgetId(): string | undefined;
  tokenRefreshed(token: RefreshedToken): void;
  /** The refresh token was refused: the user has to sign in again. */
  expired(): void;
}

type Body = { json: unknown } | { form: FormData } | undefined;

const PUBLIC_AUTH_ENDPOINTS = new Set(['auth/google', 'auth/refresh', 'auth/demo']);

class ApiClient {
  private readonly baseUrl = config.apiBaseUrl;
  private session: ApiSession | null = null;
  private refreshPromise: Promise<string> | null = null;

  connect(session: ApiSession) {
    this.session = session;
  }

  private headers(endpoint: string, body: Body, accessToken?: string): Record<string, string> {
    // A form leaves Content-Type unset so the browser adds the boundary itself.
    const headers: Record<string, string> = body && 'form' in body ? {} : { 'Content-Type': 'application/json' };
    const token = accessToken ?? this.session?.accessToken();
    if (token && !PUBLIC_AUTH_ENDPOINTS.has(endpoint)) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    const budgetId = this.session?.budgetId();
    if (budgetId && !endpoint.includes('budgets')) {
      headers['x-budget-id'] = budgetId;
    }
    return headers;
  }

  /** Sends a request; on a 401 refreshes the access token once and retries. */
  private async send(method: string, endpoint: string, body?: Body): Promise<Response> {
    const request = (accessToken?: string) =>
      fetch(`${this.baseUrl}/${endpoint}`, {
        method,
        headers: this.headers(endpoint, body, accessToken),
        body: body ? ('form' in body ? body.form : JSON.stringify(body.json)) : undefined,
        credentials: 'include',
      });

    const res = await request();
    if (res.status !== 401 || endpoint === 'auth/refresh') return res;
    return request(await this.refreshSession());
  }

  private async json<T>(res: Response): Promise<T> {
    const text = await res.text();
    let data = {} as T;
    try {
      if (text) data = JSON.parse(text);
    } catch (error) {
      // an error page that isn't JSON still ends up as an ApiError below
      if (res.ok) {
        console.error('Failed to parse response:', text);
        throw error;
      }
    }
    if (!res.ok) {
      const message = (data as Record<string, unknown>)?.error ?? res.statusText;
      throw new ApiError(String(message), res.status);
    }
    return data;
  }

  /**
   * Gets a new access token with the refresh token. Concurrent callers share
   * one request. Only a 401 ends the session: a server that's down or failing
   * says nothing about it, so the user stays signed in.
   */
  refreshSession(): Promise<string> {
    this.refreshPromise ??= (async () => {
      try {
        const refreshToken = this.session?.refreshToken();
        const res = refreshToken
          ? await fetch(`${this.baseUrl}/auth/refresh`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ refreshToken }),
            })
          : null;
        if (!res || res.status === 401) {
          this.session?.expired();
          throw new ApiError('Session expired', 401);
        }
        const token = await this.json<RefreshedToken>(res);
        this.session?.tokenRefreshed(token);
        return token.accessToken;
      } finally {
        this.refreshPromise = null;
      }
    })();
    return this.refreshPromise;
  }

  async get<T>(endpoint: string): Promise<T> {
    return this.json<T>(await this.send('GET', endpoint));
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.json<T>(await this.send('DELETE', endpoint));
  }

  async post<T, B = unknown>(endpoint: string, data: B): Promise<T> {
    return this.json<T>(await this.send('POST', endpoint, { json: data }));
  }

  async patch<T, B = unknown>(endpoint: string, data: B): Promise<T> {
    return this.json<T>(await this.send('PATCH', endpoint, { json: data }));
  }

  async put<T, B = unknown>(endpoint: string, data: B): Promise<T> {
    return this.json<T>(await this.send('PUT', endpoint, { json: data }));
  }

  /** POST multipart/form-data (file uploads). */
  async postForm<T>(endpoint: string, form: FormData): Promise<T> {
    return this.json<T>(await this.send('POST', endpoint, { form }));
  }

  /**
   * Fetch a binary endpoint with auth headers. Read the file from the Blob
   * (`blob.text()`), never by fetching an object URL: the CSP's connect-src
   * doesn't allow blob: URLs.
   */
  async fetchBlob(endpoint: string): Promise<Blob> {
    const res = await this.send('GET', endpoint);
    if (!res.ok) await this.json(res);
    return res.blob();
  }

  /**
   * Fetch a binary endpoint and return an object URL for use in <img>/<a>
   * (plain src URLs can't carry the Authorization header). Callers must
   * URL.revokeObjectURL when done.
   */
  async fetchBlobUrl(endpoint: string): Promise<string> {
    return URL.createObjectURL(await this.fetchBlob(endpoint));
  }

  /**
   * Opens an authed file in a new tab. The tab is opened before the fetch,
   * while the click still counts as a user action, or the popup blocker would
   * stop it.
   */
  async openFile(endpoint: string): Promise<void> {
    const tab = window.open('', '_blank');
    try {
      const url = await this.fetchBlobUrl(endpoint);
      if (tab) tab.location.href = url;
      else window.location.href = url;
      // the tab has loaded it by then; the URL only holds the blob in memory
      window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch (err) {
      tab?.close();
      throw err;
    }
  }
}

export const apiClient = new ApiClient();
