import {
  createAsyncThunk,
  createSlice,
  type PayloadAction,
} from '@reduxjs/toolkit';
import type {
  CategoryGroup,
  CategoryGroupState,
} from '../types/category.types';
import { apiClient, LoadingState } from '@/utils';
import type { RootState } from '@/app';

const initialState: CategoryGroupState = {
  allCategoryGroups: [],
  collapseAllGroups: false,
  inflow: 0,
  loading: LoadingState.IDLE,
  error: null,
};

export const fetchAllCategoryGroups = createAsyncThunk<CategoryGroup[], string>(
  'categoryGroups/fetchAllCategoryGroups',
  async (month: string) => {
    const res = await apiClient.get<CategoryGroup[]>(
      `category-groups?month=${month}`,
    );
    return res.map((group) => {
      if (group.id === '00000000-0000-0000-0000-000000000000') {
        group.collapsed = true;
      }
      return group;
    });
  },
);

export const fetchInflowAmount = createAsyncThunk<number>(
  'categorGroups/fetchInflowAmount',
  async () => {
    return await apiClient.get<number>(`categories/inflow`);
  },
);

const categoryGroupSlice = createSlice({
  name: 'categoryGroups',
  initialState,
  reducers: {
    toggleGroupCollapse: (state, action: PayloadAction<string>) => {
      const group = state.allCategoryGroups.find(
        (group) => group.id === action.payload,
      );
      if (group) {
        group.collapsed = !group.collapsed;
      }
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllCategoryGroups.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchAllCategoryGroups.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.allCategoryGroups = action.payload ?? [];
        state.error = null;
      })
      .addCase(fetchAllCategoryGroups.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load category groups';
      })
      .addCase(fetchInflowAmount.fulfilled, (state, action) => {
        state.inflow = action.payload;
      })
      .addCase(fetchInflowAmount.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load inflow amount';
      });
  },
});

export const { toggleGroupCollapse } = categoryGroupSlice.actions;

export default categoryGroupSlice.reducer;

// selectors
export const selectCategoryGroups = (state: RootState) => state.categoryGroups;
