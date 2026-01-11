import {
  createAsyncThunk,
  createSelector,
  createSlice,
  type PayloadAction,
} from '@reduxjs/toolkit';
import type {
  CategoryState,
  Category,
  CategoryGroup,
} from '../types/category.types';
import { apiClient, LoadingState } from '@/utils';
import type { RootState } from '@/app';

const initialCategoryState: CategoryState = {
  allCategories: [],
  allCategoryGroups: [],
  collapseAllGroups: false,
  inflowAmount: 0,
  loading: LoadingState.IDLE,
  inflowLoading: LoadingState.IDLE,
  error: null,
};

export const fetchInflowAmount = createAsyncThunk<number>(
  'categories/fetchInflowAmount',
  async () => {
    return await apiClient.get<number>('categories/inflow');
  },
);

export const fetchAllCategories = createAsyncThunk<Category[]>(
  'categories/fetchAllCategories',
  async () => {
    return await apiClient.get<Category[]>('categories');
  },
);

export const fetchCategoryById = createAsyncThunk<Category, string>(
  'categories/fetchCategoryById',
  async (id: string) => {
    const res = await apiClient.get<Category>(`categories/${id}`);
    return res;
  },
);

export const updateCategoryBudget = createAsyncThunk<
  { budgeted: number },
  { budgeted: number; categoryId: string; month: string }
>(
  'categories/updateCategoryBudget',
  async ({ budgeted, categoryId, month }) => {
    console.log('updateCategoryBudget', budgeted, categoryId, month);
    await apiClient.patch(`categories/${categoryId}/${month}`, {
      budgeted: budgeted,
    });
    console.log('updateCategoryBudget.fulfilled', budgeted, categoryId, month);
    return { budgeted, categoryId, month };
  },
);

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

const categoriesSlice = createSlice({
  name: 'categories',
  initialState: initialCategoryState,
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
      .addCase(fetchInflowAmount.pending, (state) => {
        state.inflowLoading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchInflowAmount.fulfilled, (state, action) => {
        state.inflowLoading = LoadingState.SUCCESS;
        state.inflowAmount = action.payload;
        state.error = null;
      })
      .addCase(fetchInflowAmount.rejected, (state, action) => {
        state.inflowLoading = LoadingState.ERROR;
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
      })
      .addCase(updateCategoryBudget.fulfilled, (state, action) => {
        const { budgeted, categoryId, month } = action.meta.arg;
        state.loading = LoadingState.SUCCESS;

        state.allCategoryGroups = state.allCategoryGroups.map((group) => {
          return {
            ...group,
            categories: (group.categories = group.categories.map((cat) => {
              if (cat.id === categoryId) {
                const oldBudgeted = cat?.budgeted[month] ?? 0;

                cat.budgeted[month] = budgeted;
                if (cat.activity) {
                  cat.activity[month] =
                    cat.activity[month] - oldBudgeted + budgeted;
                }
                if (cat.balance) {
                  cat.balance[month] =
                    cat.balance[month] - oldBudgeted + budgeted;
                }
                return cat;
              }
              return cat;
            })),
          };
        });
        state.error = null;
      })
      .addCase(fetchCategoryById.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        const index = state.allCategories.findIndex(
          (c) => c.id === action.payload.id,
        );
        if (index !== -1) {
          state.allCategories[index] = action.payload;
        } else {
          state.allCategories.push(action.payload);
        }
        state.error = null;
      })
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
      });
  },
});

export const { toggleGroupCollapse } = categoriesSlice.actions;

export default categoriesSlice.reducer;

const selectCategoriesState = (state: RootState) => state.categories;
const selectBudgetState = (state: RootState) => state.budgets;

export const selectCategoryGroups = (state: RootState) => state.categories;
export const selectInflowAmount = (state: RootState) =>
  state.categories.inflowAmount;
export const selectCategoryLoading = (state: RootState) =>
  state.categories.loading;
export const selectInflowLoading = (state: RootState) =>
  state.categories.inflowLoading;

export const selectInflowCategory = createSelector(
  [selectCategoriesState, selectBudgetState],
  (categoriesState, budgetsState) => {
    const inflowId = budgetsState.selectedBudget?.metadata?.inflowCategoryId;
    if (!inflowId) return null;
    return categoriesState.allCategories.find((c) => c.id === inflowId) || null;
  },
);
