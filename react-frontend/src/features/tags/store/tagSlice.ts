import { apiClient, LoadingState } from '@/utils';
import type { Tag, TagState } from '../types/tag.types';
import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';

const initialState: TagState = {
  allTags: [],
  loading: LoadingState.IDLE,
  error: null,
};

export const fetchAllTags = createAsyncThunk<Tag[]>(
  'tags/fetchAllTags',
  async () => {
    return await apiClient.get<Tag[]>('tags');
  },
);

export const createTag = createAsyncThunk<Tag, Partial<Tag>>(
  'tags/createTag',
  async (tag) => {
    return await apiClient.post<Tag>('tags', tag);
  },
);

export const updateTag = createAsyncThunk<void, { id: string; tag: Partial<Tag> }>(
  'tags/updateTag',
  async ({ id, tag }) => {
    await apiClient.patch(`tags/${id}`, tag);
  },
);

export const deleteTag = createAsyncThunk<void, string>(
  'tags/deleteTag',
  async (id) => {
    await apiClient.delete(`tags/${id}`);
  },
);

const tagSlice = createSlice({
  name: 'tags',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllTags.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchAllTags.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.allTags = action.payload ?? [];
        state.error = null;
      })
      .addCase(fetchAllTags.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load tags';
      })
      .addCase(createTag.fulfilled, (state, action) => {
        state.allTags.push(action.payload);
      })
      .addCase(deleteTag.fulfilled, (state, action) => {
        state.allTags = state.allTags.filter((t) => t.id !== action.meta.arg);
      });
  },
});

export default tagSlice.reducer;
