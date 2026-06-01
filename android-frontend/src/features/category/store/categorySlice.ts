import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { LoadingState } from '../../../utils/constants';
import type { Category, CategoryGroup, CategoryState } from '../types';

const initialState: CategoryState = {
  allCategories: [],
  allCategoryGroups: [],
  inflowAmount: 0,
  loading: LoadingState.IDLE,
  inflowLoading: LoadingState.IDLE,
  error: null
};

export const fetchInflowAmount = createAsyncThunk<number>('categories/fetchInflowAmount', async () => {
  return apiClient.get<number>('categories/inflow');
});

export const fetchAllCategories = createAsyncThunk<Category[]>('categories/fetchAllCategories', async () => {
  return apiClient.get<Category[]>('categories');
});

export const fetchCategoryById = createAsyncThunk<Category, string>('categories/fetchCategoryById', async (id) => {
  return apiClient.get<Category>(`categories/${id}`);
});

export const fetchAllCategoryGroups = createAsyncThunk<CategoryGroup[], string>(
  'categoryGroups/fetchAllCategoryGroups',
  async (month) => {
    const groups = await apiClient.get<CategoryGroup[]>(`category-groups?month=${month}`);
    return groups.map((group) => ({
      ...group,
      collapsed: group.id === '00000000-0000-0000-0000-000000000000' ? true : group.collapsed
    }));
  }
);

export const updateCategoryBudget = createAsyncThunk<
  { budgeted: number; categoryId: string; month: string },
  { budgeted: number; categoryId: string; month: string }
>('categories/updateCategoryBudget', async ({ budgeted, categoryId, month }) => {
  await apiClient.patch(`categories/${categoryId}/${month}`, { budgeted });
  return { budgeted, categoryId, month };
});

const categorySlice = createSlice({
  name: 'categories',
  initialState,
  reducers: {
    toggleGroupCollapse: (state, action: PayloadAction<string>) => {
      const group = state.allCategoryGroups.find((item) => item.id === action.payload);
      if (group) group.collapsed = !group.collapsed;
    }
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchInflowAmount.pending, (state) => {
        state.inflowLoading = LoadingState.PENDING;
      })
      .addCase(fetchInflowAmount.fulfilled, (state, action) => {
        state.inflowLoading = LoadingState.SUCCESS;
        state.inflowAmount = action.payload;
      })
      .addCase(fetchInflowAmount.rejected, (state, action) => {
        state.inflowLoading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load inflow';
      })
      .addCase(fetchAllCategories.fulfilled, (state, action) => {
        state.allCategories = action.payload;
      })
      .addCase(fetchCategoryById.fulfilled, (state, action) => {
        const index = state.allCategories.findIndex((category) => category.id === action.payload.id);
        if (index >= 0) state.allCategories[index] = action.payload;
        else state.allCategories.push(action.payload);
      })
      .addCase(fetchAllCategoryGroups.pending, (state) => {
        state.loading = LoadingState.PENDING;
      })
      .addCase(fetchAllCategoryGroups.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.allCategoryGroups = action.payload ?? [];
      })
      .addCase(fetchAllCategoryGroups.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load categories';
      })
      .addCase(updateCategoryBudget.fulfilled, (state, action) => {
        const { categoryId, month, budgeted } = action.payload;
        for (const group of state.allCategoryGroups) {
          for (const category of group.categories) {
            if (category.id !== categoryId) continue;
            const previous = category.budgeted?.[month] ?? 0;
            category.budgeted = { ...(category.budgeted ?? {}), [month]: budgeted };
            category.balance = { ...(category.balance ?? {}), [month]: (category.balance?.[month] ?? 0) - previous + budgeted };
          }
        }
      });
  }
});

export const { toggleGroupCollapse } = categorySlice.actions;
export default categorySlice.reducer;
