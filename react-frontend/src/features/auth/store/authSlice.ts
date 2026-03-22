import { createSlice, createAsyncThunk, type PayloadAction } from '@reduxjs/toolkit';
import type { RootState } from '@/app';
import { LoadingState } from '@/utils';
import type {
  AuthState,
  AuthTokens,
  LoginResponse,
  User,
} from '../types/auth.types';
import { apiClient } from '@/utils';

const AUTH_STORAGE_KEY = 'pennywise_auth';

// Helper to load auth state from localStorage
const loadAuthFromStorage = (): { user: User | null; tokens: AuthTokens | null } => {
  try {
    const stored = localStorage.getItem(AUTH_STORAGE_KEY);
    if (stored) {
      const data = JSON.parse(stored);
      // Check if token is expired
      if (data.tokens && data.tokens.expiresAt > Date.now()) {
        return data;
      }
      // Clear expired auth
      localStorage.removeItem(AUTH_STORAGE_KEY);
    }
  } catch (e) {
    console.error('Error loading auth from storage:', e);
  }
  return { user: null, tokens: null };
};

// Helper to save auth state to localStorage
const saveAuthToStorage = (user: User, tokens: AuthTokens) => {
  try {
    localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify({ user, tokens }));
  } catch (e) {
    console.error('Error saving auth to storage:', e);
  }
};

// Helper to clear auth from localStorage
const clearAuthFromStorage = () => {
  try {
    localStorage.removeItem(AUTH_STORAGE_KEY);
  } catch (e) {
    console.error('Error clearing auth from storage:', e);
  }
};

// Initial state with data from localStorage
const storedAuth = loadAuthFromStorage();

const initialState: AuthState = {
  user: storedAuth.user,
  tokens: storedAuth.tokens,
  isAuthenticated: !!storedAuth.tokens,
  loading: LoadingState.IDLE,
  error: null,
};

// Async thunk to authenticate with Google
export const loginWithGoogle = createAsyncThunk<
  LoginResponse,
  string, // Google credential (JWT token)
  { rejectValue: string }
>('auth/loginWithGoogle', async (credential, { rejectWithValue }) => {
  try {
    const response = await apiClient.post<LoginResponse>('auth/google', {
      credential,
    } as unknown as Partial<LoginResponse>);
    return response;
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Login failed';
    return rejectWithValue(message);
  }
});

// Async thunk to refresh access token
export const refreshAccessToken = createAsyncThunk<
  { accessToken: string; expiresIn: number },
  void,
  { state: RootState; rejectValue: string }
>('auth/refreshToken', async (_, { getState, rejectWithValue }) => {
  try {
    const { tokens } = getState().auth;
    if (!tokens?.refreshToken) {
      return rejectWithValue('No refresh token available');
    }
    const response = await apiClient.post<{ accessToken: string; expiresIn: number }>(
      'auth/refresh',
      { refreshToken: tokens.refreshToken } as unknown as Partial<{ accessToken: string; expiresIn: number }>
    );
    return response;
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Token refresh failed';
    return rejectWithValue(message);
  }
});

// Async thunk to logout
export const logout = createAsyncThunk<void, void, { state: RootState }>(
  'auth/logout',
  async (_, { getState }) => {
    try {
      const { tokens } = getState().auth;
      if (tokens?.refreshToken) {
        await apiClient.post('auth/logout', { refreshToken: tokens.refreshToken });
      }
    } catch (error) {
      // Ignore logout errors - we'll clear local state anyway
      console.error('Logout error:', error);
    }
  }
);

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    // Clear any auth errors
    clearError: (state) => {
      state.error = null;
    },
    // Reset auth state (for local-only logout scenarios)
    resetAuth: (state) => {
      state.user = null;
      state.tokens = null;
      state.isAuthenticated = false;
      state.loading = LoadingState.IDLE;
      state.error = null;
      clearAuthFromStorage();
    },
  },
  extraReducers: (builder) => {
    // Login with Google
    builder
      .addCase(loginWithGoogle.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(loginWithGoogle.fulfilled, (state, action: PayloadAction<LoginResponse>) => {
        const { user, accessToken, refreshToken, expiresIn } = action.payload;
        const tokens: AuthTokens = {
          accessToken,
          refreshToken,
          expiresAt: Date.now() + expiresIn * 1000,
        };
        state.user = user;
        state.tokens = tokens;
        state.isAuthenticated = true;
        state.loading = LoadingState.SUCCESS;
        state.error = null;
        saveAuthToStorage(user, tokens);
      })
      .addCase(loginWithGoogle.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.payload ?? 'Login failed';
        state.isAuthenticated = false;
      });

    // Refresh token
    builder
      .addCase(refreshAccessToken.fulfilled, (state, action) => {
        if (state.tokens) {
          state.tokens.accessToken = action.payload.accessToken;
          state.tokens.expiresAt = Date.now() + action.payload.expiresIn * 1000;
          if (state.user) {
            saveAuthToStorage(state.user, state.tokens);
          }
        }
      })
      .addCase(refreshAccessToken.rejected, (state) => {
        // Token refresh failed - clear auth state
        state.user = null;
        state.tokens = null;
        state.isAuthenticated = false;
        clearAuthFromStorage();
      });

    // Logout
    builder
      .addCase(logout.pending, (state) => {
        state.loading = LoadingState.PENDING;
      })
      .addCase(logout.fulfilled, (state) => {
        state.user = null;
        state.tokens = null;
        state.isAuthenticated = false;
        state.loading = LoadingState.IDLE;
        state.error = null;
        clearAuthFromStorage();
      })
      .addCase(logout.rejected, (state) => {
        // Even if logout fails on server, clear local state
        state.user = null;
        state.tokens = null;
        state.isAuthenticated = false;
        state.loading = LoadingState.IDLE;
        clearAuthFromStorage();
      });
  },
});

// Selectors
export const selectAuth = (state: RootState) => state.auth;
export const selectUser = (state: RootState) => state.auth.user;
export const selectIsAuthenticated = (state: RootState) => state.auth.isAuthenticated;
export const selectAuthLoading = (state: RootState) => state.auth.loading;
export const selectAuthError = (state: RootState) => state.auth.error;
export const selectAccessToken = (state: RootState) => state.auth.tokens?.accessToken;

export const { clearError, resetAuth } = authSlice.actions;
export default authSlice.reducer;
