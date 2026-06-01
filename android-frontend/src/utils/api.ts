import type { RootState } from '../app/store';
import { config } from '../config/env';

type AppDispatch = (action: unknown) => unknown;

class ApiClient {
  private readonly baseUrl = config.apiBaseUrl;
  private getState: (() => RootState) | null = null;
  private dispatch: AppDispatch | null = null;
  private refreshPromise: Promise<string> | null = null;

  setGetState(getState: () => RootState) {
    this.getState = getState;
  }

  setDispatch(dispatch: AppDispatch) {
    this.dispatch = dispatch;
  }

  private isPublicAuthEndpoint(endpoint: string) {
    return endpoint === 'auth/google' || endpoint === 'auth/refresh';
  }

  private isRefreshEndpoint(endpoint: string) {
    return endpoint === 'auth/refresh';
  }

  private getHeaders(endpoint: string): HeadersInit {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json'
    };

    const state = this.getState?.();
    const accessToken = state?.auth.tokens?.accessToken;
    if (accessToken && !this.isPublicAuthEndpoint(endpoint)) {
      headers.Authorization = `Bearer ${accessToken}`;
    }

    const selectedBudget = state?.budgets.selectedBudget;
    if (selectedBudget?.id && !endpoint.includes('budgets')) {
      headers['x-budget-id'] = selectedBudget.id;
    }

    return headers;
  }

  private async tryRefreshToken(): Promise<string> {
    if (this.refreshPromise) return this.refreshPromise;

    this.refreshPromise = (async () => {
      const refreshToken = this.getState?.().auth.tokens?.refreshToken;
      if (!refreshToken) throw new Error('No refresh token');

      const res = await fetch(`${this.baseUrl}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refreshToken })
      });

      if (!res.ok) throw new Error('Refresh failed');
      const data = (await res.json()) as { accessToken: string; expiresIn: number };
      if (this.dispatch) {
        const { refreshAccessToken } = await import('../features/auth/store/authSlice');
        this.dispatch(refreshAccessToken.fulfilled(data, '', undefined));
      }
      return data.accessToken;
    })().finally(() => {
      this.refreshPromise = null;
    });

    return this.refreshPromise;
  }

  private async parseResponse<T>(res: Response): Promise<T> {
    const text = await res.text();
    const data = text ? JSON.parse(text) : {};
    if (!res.ok) {
      throw new Error(String(data?.error ?? res.statusText));
    }
    return data as T;
  }

  private async handleResponse<T>(
    res: Response,
    method: string,
    endpoint: string,
    body?: unknown
  ): Promise<T> {
    if (res.status === 401 && !this.isRefreshEndpoint(endpoint)) {
      const newAccessToken = await this.tryRefreshToken();
      const headers = this.getHeaders(endpoint) as Record<string, string>;
      headers.Authorization = `Bearer ${newAccessToken}`;
      const retryRes = await fetch(`${this.baseUrl}/${endpoint}`, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body)
      });
      return this.parseResponse<T>(retryRes);
    }
    return this.parseResponse<T>(res);
  }

  async get<T>(endpoint: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'GET',
      headers: this.getHeaders(endpoint)
    });
    return this.handleResponse<T>(res, 'GET', endpoint);
  }

  async post<T>(endpoint: string, data: unknown): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'POST',
      headers: this.getHeaders(endpoint),
      body: JSON.stringify(data)
    });
    return this.handleResponse<T>(res, 'POST', endpoint, data);
  }

  async patch<T>(endpoint: string, data: unknown): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'PATCH',
      headers: this.getHeaders(endpoint),
      body: JSON.stringify(data)
    });
    return this.handleResponse<T>(res, 'PATCH', endpoint, data);
  }

  async delete<T>(endpoint: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'DELETE',
      headers: this.getHeaders(endpoint)
    });
    return this.handleResponse<T>(res, 'DELETE', endpoint);
  }

  async probeRoot(): Promise<{ status: number; body: string; url: string }> {
    const url = this.baseUrl.replace(/\/$/, '');
    const res = await fetch(url, { method: 'GET' });
    return {
      status: res.status,
      body: await res.text(),
      url
    };
  }
}

export const apiClient = new ApiClient();
