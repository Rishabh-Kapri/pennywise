import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { LoadingState } from '../../../utils/constants';
import { clearAuthFromStorage, loadAuthFromStorage, saveAuthToStorage } from '../../../utils/storage';
import type { RootState } from '../../../app/store';
import type { AuthState, GoogleLoginRequest, LoginResponse, RefreshTokenResponse, AuthTokens, User } from '../types';

const initialState: AuthState = {
  user: null,
  tokens: null,
  isAuthenticated: false,
  loading: LoadingState.IDLE,
  hydrated: false,
  error: null
};

export const hydrateAuth = createAsyncThunk<{ user: User | null; tokens: AuthTokens | null }>(
  'auth/hydrateAuth',
  async () => loadAuthFromStorage()
);

export const loginWithGoogle = createAsyncThunk<LoginResponse, GoogleLoginRequest, { rejectValue: string }>(
  'auth/loginWithGoogle',
  async (request, { rejectWithValue }) => {
    try {
      const response = await apiClient.post<LoginResponse>('auth/google', request);
      const tokens = {
        accessToken: response.accessToken,
        refreshToken: response.refreshToken,
        expiresAt: Date.now() + response.expiresIn * 1000
      };
      await saveAuthToStorage(response.user, tokens);
      return response;
    } catch (error) {
      return rejectWithValue(error instanceof Error ? error.message : 'Login failed');
    }
  }
);

export const refreshAccessToken = createAsyncThunk<RefreshTokenResponse, void, { state: RootState; rejectValue: string }>(
  'auth/refreshAccessToken',
  async (_, { getState, rejectWithValue }) => {
    try {
      const refreshToken = getState().auth.tokens?.refreshToken;
      if (!refreshToken) return rejectWithValue('No refresh token available');
      const response = await apiClient.post<RefreshTokenResponse>('auth/refresh', { refreshToken });
      const user = getState().auth.user;
      const currentRefreshToken = getState().auth.tokens?.refreshToken;
      if (user && currentRefreshToken) {
        await saveAuthToStorage(user, {
          accessToken: response.accessToken,
          refreshToken: currentRefreshToken,
          expiresAt: Date.now() + response.expiresIn * 1000
        });
      }
      return response;
    } catch (error) {
      return rejectWithValue(error instanceof Error ? error.message : 'Token refresh failed');
    }
  }
);

export const logout = createAsyncThunk('auth/logout', async (_, { getState }) => {
  const state = getState() as RootState;
  try {
    if (state.auth.tokens?.refreshToken) {
      await apiClient.post('auth/logout', { refreshToken: state.auth.tokens.refreshToken });
    }
  } catch {
    // API route is currently optional; local logout still succeeds.
  }
  await clearAuthFromStorage();
});

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    resetAuth: (state) => {
      state.user = null;
      state.tokens = null;
      state.isAuthenticated = false;
      state.error = null;
    }
  },
  extraReducers: (builder) => {
    builder
      .addCase(hydrateAuth.fulfilled, (state, action) => {
        state.user = action.payload.user;
        state.tokens = action.payload.tokens;
        state.isAuthenticated = Boolean(action.payload.tokens);
        state.hydrated = true;
      })
      .addCase(hydrateAuth.rejected, (state) => {
        state.hydrated = true;
      })
      .addCase(loginWithGoogle.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(loginWithGoogle.fulfilled, (state, action: PayloadAction<LoginResponse>) => {
        state.loading = LoadingState.SUCCESS;
        state.user = action.payload.user;
        state.tokens = {
          accessToken: action.payload.accessToken,
          refreshToken: action.payload.refreshToken,
          expiresAt: Date.now() + action.payload.expiresIn * 1000
        };
        state.isAuthenticated = true;
      })
      .addCase(loginWithGoogle.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.payload ?? 'Login failed';
      })
      .addCase(refreshAccessToken.fulfilled, (state, action) => {
        if (!state.tokens) return;
        state.tokens.accessToken = action.payload.accessToken;
        state.tokens.expiresAt = Date.now() + action.payload.expiresIn * 1000;
      })
      .addCase(refreshAccessToken.rejected, (state) => {
        state.user = null;
        state.tokens = null;
        state.isAuthenticated = false;
      })
      .addCase(logout.fulfilled, (state) => {
        state.user = null;
        state.tokens = null;
        state.isAuthenticated = false;
        state.loading = LoadingState.IDLE;
      });
  }
});

export const { resetAuth } = authSlice.actions;
export default authSlice.reducer;

export const selectAccessToken = (state: RootState) => state.auth.tokens?.accessToken;
