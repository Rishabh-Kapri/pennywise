import type { RootState } from '@/app';
import { config } from '@/config/env';

class ApiClient {
  private baseUrl: string;
  private getState: (() => RootState) | null = null;

  constructor() {
    this.baseUrl = config.apiBaseUrl;
  }

  setGetState(getState: () => RootState) {
    this.getState = getState;
  }

  private getHeaders(endpoint: string): HeadersInit {
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
    };

    if (this.getState) {
      const state = this.getState();

      const selectedBudget = state.budgets.selectedBudget;
      if (selectedBudget?.id && !endpoint.includes('budgets')) {
        headers['x-budget-id'] = selectedBudget.id;
      }
    }

    return headers;
  }

  async get<T>(endpoint: string): Promise<T> {
    console.log('fetching from', endpoint)
    console.log('headers', this.getHeaders(endpoint))
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'GET',
      headers: this.getHeaders(endpoint),
    });
    return res.json();
  }

  async post<T>(endpoint: string, data: Partial<T>): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'POST',
      headers: this.getHeaders(endpoint),
      body: JSON.stringify(data),
    });
    return res.json();
  }

  async delete<T>(endpoint: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'DELETE',
      headers: this.getHeaders(endpoint),
    });
    return res.json();
  }

  async patch<T>(endpoint: string, data: Partial<T>): Promise<T> {
    const res = await fetch(`${this.baseUrl}/${endpoint}`, {
      method: 'PATCH',
      headers: this.getHeaders(endpoint),
      body: JSON.stringify(data),
    });
    return res.json();
  }
}

export const apiClient = new ApiClient();
