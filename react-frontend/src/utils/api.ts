import type { RootState } from '@/app';
import { config } from '@/config/env';

type AppDispatch = (action: unknown) => unknown;

class ApiClient {
  private baseUrl: string;
  private getState: (() => RootState) | null = null;
  private dispatch: AppDispatch | null = null;
  private refreshPromise: Promise<string> | null = null;

  constructor() {
    this.baseUrl = config.apiBaseUrl;
  }

  setGetState(getState: () => RootState) {
    this.getState = getState;
  }

  setDispatch(dispatch: AppDispatch) {
    this.dispatch = dispatch;
  }

  private getHeaders(endpoint: string): HeadersInit {
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
    };

    if (this.getState) {
      const state = this.getState();

      // Add Authorization header if user is authenticated
      const accessToken = state.auth?.tokens?.accessToken;
      if (accessToken && !endpoint.includes('auth/')) {
        headers['Authorization'] = `Bearer ${accessToken}`;
      }

      const selectedBudget = state.budgets.selectedBudget;
      if (selectedBudget?.id && !endpoint.includes('budgets')) {
        headers['x-budget-id'] = selectedBudget.id;
      }
    }

    return headers;
  }

  private isAuthEndpoint(endpoint: string): boolean {
    return endpoint.includes('auth/');
  }

  private async tryRefreshToken(): Promise<string> {
    // If a refresh is already in flight, reuse it (prevents parallel refresh calls)
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    this.refreshPromise = (async () => {
      try {
        const state = this.getState?.();
        const refreshToken = state?.auth?.tokens?.refreshToken;
        if (!refreshToken) {
          throw new Error('No refresh token');
        }

        const res = await fetch(`${this.baseUrl}/auth/refresh`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refreshToken }),
        });

        if (!res.ok) {
          throw new Error('Refresh failed');
        }

        const data = await res.json();
        // Dispatch the refresh success to update store + localStorage
        if (this.dispatch) {
          const { refreshAccessToken } = await import('@/features/auth/store/authSlice');
          this.dispatch(refreshAccessToken.fulfilled(data, '', undefined));
        }
        return data.accessToken as string;
      } catch {
        // Refresh failed — clear auth and redirect to login
        if (this.dispatch) {
          const { resetAuth } = await import('@/features/auth/store/authSlice');
          this.dispatch(resetAuth());
        }
        window.location.href = '/login';
        throw new Error('Session expired');
      } finally {
        this.refreshPromise = null;
      }
    })();

    return this.refreshPromise;
  }

  private async handleResponse<T>(res: Response, method: string, endpoint: string, body?: unknown): Promise<T> {
    // On 401 for non-auth endpoints, try refreshing the token and retry once
    if (res.status === 401 && !this.isAuthEndpoint(endpoint)) {
      const newAccessToken = await this.tryRefreshToken();

      const headers = this.getHeaders(endpoint) as Record<string, string>;
      headers['Authorization'] = `Bearer ${newAccessToken}`;

      const retryRes = await fetch(`${this.baseUrl}/${endpoint}`, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
      });
      return this.parseResponse<T>(retryRes);
    }

    return this.parseResponse<T>(res);
  }

  private async parseResponse<T>(res: Response): Promise<T> {
    const text = await res.text();
    let data: T;
    try {
      data = text ? JSON.parse(text) : ({} as T);
    } catch (error) {
      console.error('Failed to parse response:', text);
      throw error;
    }
    if (!res.ok) {
      const message =
        (data as Record<string, unknown>)?.error ?? res.statusText;
      throw new Error(String(message));
    }
    return data;
  }

  async get<T>(endpoint: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'GET',
      headers: this.getHeaders(endpoint),
    });
    return this.handleResponse<T>(res, 'GET', endpoint);
  }

  async post<T>(endpoint: string, data: Partial<T>): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'POST',
      headers: this.getHeaders(endpoint),
      body: JSON.stringify(data),
    });
    return this.handleResponse<T>(res, 'POST', endpoint, data);
  }

  async delete<T>(endpoint: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'DELETE',
      headers: this.getHeaders(endpoint),
    });
    return this.handleResponse<T>(res, 'DELETE', endpoint);
  }

  async patch<T>(endpoint: string, data: Partial<T>): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'PATCH',
      headers: this.getHeaders(endpoint),
      body: JSON.stringify(data),
    });
    return this.handleResponse<T>(res, 'PATCH', endpoint, data);
  }
}

export const apiClient = new ApiClient();
