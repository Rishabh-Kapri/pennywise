import type { LoadingState } from '../../utils/constants';

export interface User {
  id: string;
  googleId?: string;
  email: string;
  name: string;
  picture?: string;
}

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  expiresAt: number;
}

export interface LoginResponse {
  user: User;
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export interface GoogleLoginRequest {
  code: string;
  redirectUri?: string;
  codeVerifier?: string;
}

export interface RefreshTokenResponse {
  accessToken: string;
  expiresIn: number;
}

export interface AuthState {
  user: User | null;
  tokens: AuthTokens | null;
  isAuthenticated: boolean;
  loading: LoadingState;
  hydrated: boolean;
  error: string | null;
}
