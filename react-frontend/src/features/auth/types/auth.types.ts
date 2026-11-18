import type { LoadingState } from '@/utils';

export interface User {
  id: string;
  googleId: string;
  email: string;
  name: string;
  picture?: string;
}

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  expiresAt: number; // timestamp
}

export interface AuthState {
  user: User | null;
  tokens: AuthTokens | null;
  loading: LoadingState;
  error: string | null;
}

export interface LoginResponse {
  user: User;
  accessToken: string;
  refreshToken: string;
  expiresIn: number; // seconds
}
