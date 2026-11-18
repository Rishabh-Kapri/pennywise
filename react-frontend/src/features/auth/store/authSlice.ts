import {
  createSlice,
  createAsyncThunk,
  type PayloadAction,
} from '@reduxjs/toolkit';
import type { RootState } from '@/app';
import { LoadingState } from '@/utils';
import type {
  AuthState,
  AuthTokens,
  LoginResponse,
} from '../types/auth.types';
import { apiClient, type RefreshedToken } from '@/utils/api';

const AUTH_STORAGE_KEY = 'pennywise_auth';

function loadSession(): Pick<AuthState, 'user' | 'tokens'> {
  try {
    const stored = localStorage.getItem(AUTH_STORAGE_KEY);
    if (stored) {
      const data = JSON.parse(stored);
      // An expired access token is fine: apiClient refreshes it with the
      // refresh token on the first 401.
      if (data.tokens?.refreshToken) {
        return data;
      }
      localStorage.removeItem(AUTH_STORAGE_KEY);
    }
  } catch (e) {
    console.error('Error loading auth from storage:', e);
  }
  return { user: null, tokens: null };
}

/** Keeps localStorage in step with the session; the store calls it on every change. */
export function saveSession({ user, tokens }: AuthState) {
  try {
    if (user && tokens) localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify({ user, tokens }));
    else localStorage.removeItem(AUTH_STORAGE_KEY);
  } catch (e) {
    console.error('Error saving auth to storage:', e);
  }
}

const initialState: AuthState = {
  ...loadSession(),
  loading: LoadingState.IDLE,
  error: null,
};

// Async thunk to authenticate with Google
export const loginWithGoogle = createAsyncThunk<
  LoginResponse,
  string, // Google credential (JWT token)
  { rejectValue: string }
>('auth/loginWithGoogle', async (code, { rejectWithValue }) => {
  try {
    const response = await apiClient.post<LoginResponse>('auth/google', {
      code,
    });
    return response;
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Login failed';
    return rejectWithValue(message);
  }
});

// Async thunk to log into the seeded demo account (no Google required)
export const loginAsDemo = createAsyncThunk<
  LoginResponse,
  void,
  { rejectValue: string }
>('auth/loginAsDemo', async (_, { rejectWithValue }) => {
  try {
    const response = await apiClient.post<LoginResponse>(
      'auth/demo',
      {},
    );
    return response;
  } catch (error) {
    const message =
      error instanceof Error ? error.message : 'Demo login failed';
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
        await apiClient.post('auth/logout', {
          refreshToken: tokens.refreshToken,
        });
      }
    } catch (error) {
      // Ignore logout errors - we'll clear local state anyway
      console.error('Logout error:', error);
    }
  },
);

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    /** apiClient got a new access token. */
    tokenRefreshed: (state, action: PayloadAction<RefreshedToken>) => {
      if (!state.tokens) return;
      state.tokens.accessToken = action.payload.accessToken;
      state.tokens.expiresAt = Date.now() + action.payload.expiresIn * 1000;
    },
    /** Signed out, here or because the session ended on the server. */
    sessionEnded: () => ({ user: null, tokens: null, loading: LoadingState.IDLE, error: null }),
  },
  extraReducers: (builder) => {
    // Shared login handlers (Google and demo logins behave identically)
    const loginPending = (state: AuthState) => {
      state.loading = LoadingState.PENDING;
      state.error = null;
    };
    const loginFulfilled = (
      state: AuthState,
      action: PayloadAction<LoginResponse>,
    ) => {
      const { user, accessToken, refreshToken, expiresIn } = action.payload;
      const tokens: AuthTokens = {
        accessToken,
        refreshToken,
        expiresAt: Date.now() + expiresIn * 1000,
      };
      state.user = user;
      state.tokens = tokens;
      state.loading = LoadingState.SUCCESS;
      state.error = null;
    };
    const loginRejected = (
      state: AuthState,
      action: { payload?: string },
    ) => {
      state.loading = LoadingState.ERROR;
      state.error = action.payload ?? 'Login failed';
    };

    // Login with Google
    builder
      .addCase(loginWithGoogle.pending, loginPending)
      .addCase(loginWithGoogle.fulfilled, loginFulfilled)
      .addCase(loginWithGoogle.rejected, loginRejected);

    // Login as demo user
    builder
      .addCase(loginAsDemo.pending, loginPending)
      .addCase(loginAsDemo.fulfilled, loginFulfilled)
      .addCase(loginAsDemo.rejected, loginRejected);

    // logout swallows server errors: the local session ends either way.
    builder
      .addCase(logout.pending, (state) => {
        state.loading = LoadingState.PENDING;
      })
      .addCase(logout.fulfilled, () => ({ user: null, tokens: null, loading: LoadingState.IDLE, error: null }));
  },
});

// Selectors
export const selectUser = (state: RootState) => state.auth.user;
export const selectIsAuthenticated = (state: RootState) => !!state.auth.tokens;
export const selectAuthLoading = (state: RootState) => state.auth.loading;
export const selectAuthError = (state: RootState) => state.auth.error;
export const selectAccessToken = (state: RootState) =>
  state.auth.tokens?.accessToken;
/** The shared demo account: destructive/config actions are disabled for it. */
export const DEMO_USER_EMAIL = 'demo@pennywise.local';
export const selectIsDemoUser = (state: RootState) =>
  state.auth.user?.email === DEMO_USER_EMAIL;

export const { tokenRefreshed, sessionEnded } = authSlice.actions;
export default authSlice.reducer;
