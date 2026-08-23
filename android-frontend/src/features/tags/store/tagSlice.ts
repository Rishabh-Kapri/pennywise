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

export const createTag = createAsyncThunk<Tag, Partial<Tag>>('tags/createTag', async (tag) => {
  return apiClient.post<Tag>('tags', tag);
});

export const updateTag = createAsyncThunk<
  { id: string; tag: Partial<Tag> },
  { id: string; tag: Partial<Tag> }
>('tags/updateTag', async ({ id, tag }) => {
  await apiClient.patch(`tags/${id}`, tag);
  return { id, tag };
});

export const deleteTag = createAsyncThunk<void, string>('tags/deleteTag', async (id) => {
  await apiClient.delete(`tags/${id}`);
});

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
        state.tags = action.payload ?? [];
        state.error = null;
      })
      .addCase(fetchAllTags.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load tags';
      })
      .addCase(createTag.fulfilled, (state, action) => {
        state.tags.push(action.payload);
        state.error = null;
      })
      .addCase(createTag.rejected, (state, action) => {
        state.error = action.error.message ?? 'Failed to create tag';
      })
      .addCase(updateTag.fulfilled, (state, action) => {
        const existing = state.tags.find((tag) => tag.id === action.payload.id);
        if (existing) {
          Object.assign(existing, action.payload.tag);
        }
        state.error = null;
      })
      .addCase(updateTag.rejected, (state, action) => {
        state.error = action.error.message ?? 'Failed to update tag';
      })
      .addCase(deleteTag.fulfilled, (state, action) => {
        state.tags = state.tags.filter((tag) => tag.id !== action.meta.arg);
        state.error = null;
      })
      .addCase(deleteTag.rejected, (state, action) => {
        state.error = action.error.message ?? 'Failed to delete tag';
      });
  }
});

export default tagSlice.reducer;

export const selectAllTags = (state: { tags: TagState }) => state.tags.tags;
export const selectTagsLoading = (state: { tags: TagState }) => state.tags.loading;
export const selectTagsError = (state: { tags: TagState }) => state.tags.error;
