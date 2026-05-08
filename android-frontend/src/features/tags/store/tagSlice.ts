import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { LoadingState } from '../../../utils/constants';
import type { Tag, TagState } from '../types';

const initialState: TagState = {
  tags: [],
  loading: LoadingState.IDLE,
  error: null
};

export const fetchAllTags = createAsyncThunk<Tag[]>('tags/fetchAllTags', async () => {
  return apiClient.get<Tag[]>('tags');
});

const tagSlice = createSlice({
  name: 'tags',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllTags.pending, (state) => {
        state.loading = LoadingState.PENDING;
      })
      .addCase(fetchAllTags.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.tags = action.payload ?? [];
      })
      .addCase(fetchAllTags.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load tags';
      });
  }
});

export default tagSlice.reducer;
