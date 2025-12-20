import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import type { CategoryState, Category } from '../types/category.types';
import { apiClient, LoadingState } from '@/utils';

const initialCategoryState: CategoryState = {
  allCategories: [],
  inflowAmount: 0,
  loading: LoadingState.IDLE,
  error: null,
};

export const fetchInflowAmount = createAsyncThunk<number>(
  'categories/fetchInflowAmount',
  async () => {
    return await apiClient.get('categories/inflow');
  },
);

export const fetchAllCategories = createAsyncThunk<Category[]>(
  'categories/fetchAllCategories',
  async () => {
    return await apiClient.get('categories');
  },
);

const categoriesSlice = createSlice({
  name: 'categories',
  initialState: initialCategoryState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchInflowAmount.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchInflowAmount.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.inflowAmount = action.payload;
        state.error = null;
      })
      .addCase(fetchInflowAmount.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.inflowAmount = 0;
        state.error = action.error.message ?? null;
      })
      .addCase(fetchAllCategories.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchAllCategories.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.allCategories = action.payload;
        state.error = null;
      })
      .addCase(fetchAllCategories.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.allCategories = [];
        state.error = action.error.message ?? 'Failed to load categories';
      });
  },
});

export default categoriesSlice.reducer;
