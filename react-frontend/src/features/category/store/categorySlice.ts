import {
  createAsyncThunk,
  createSelector,
  createSlice,
  type PayloadAction,
} from '@reduxjs/toolkit';
import type {
  CategoryState,
  Category,
  CategoryGoal,
  CategoryGroup,
} from '../types/category.types';
import { apiClient, LoadingState } from '@/utils';
import type { RootState } from '@/app';

interface CreateCategoryPayload {
  name: string;
  budgetId: string;
  categoryGroupId: string;
  note?: string | null;
  hidden?: boolean;
  budgeted?: Record<string, number>;
}

interface CreateCategoryResponse {
  id: string;
  name: string;
  categoryGroupId: string;
}

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

export const createCategory = createAsyncThunk<
  CreateCategoryResponse,
  CreateCategoryPayload
>('categories/createCategory', async (category) => {
  return await apiClient.post<CreateCategoryResponse>('categories', category);
});

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

export interface CategoryDetails {
  note?: string;
  description?: string;
}

export const updateCategoryDetails = createAsyncThunk<
  CategoryDetails,
  { categoryId: string } & CategoryDetails
>('categories/updateCategoryDetails', async ({ categoryId, ...details }) => {
  return await apiClient.put<CategoryDetails>(`categories/${categoryId}/details`, details);
});

// Only drafts a description; nothing is stored until updateCategoryDetails.
export async function suggestCategoryDescription(categoryId: string): Promise<string> {
  const res = await apiClient.post<{ description: string }, Record<string, never>>(
    `categories/${categoryId}/description/suggest`,
    {},
  );
  return res.description;
}

// The server returns the goal as stored, with fields its type does not use cleared.
export const updateCategoryGoal = createAsyncThunk<
  CategoryGoal,
  { categoryId: string; goal: CategoryGoal }
>('categories/updateCategoryGoal', async ({ categoryId, goal }) => {
  return await apiClient.put<CategoryGoal>(`categories/${categoryId}/goal`, goal);
});

export const deleteCategoryGoal = createAsyncThunk<void, string>(
  'categories/deleteCategoryGoal',
  async (categoryId) => {
    await apiClient.delete(`categories/${categoryId}/goal`);
  },
);

// The API lists hidden categories under this synthetic group, whatever group they belong to.
export const HIDDEN_GROUP_ID = '00000000-0000-0000-0000-000000000000';

export type CategoryChanges = Partial<Pick<Category, 'name' | 'hidden' | 'categoryGroupId'>>;

// PUT /categories/:id replaces the row (name, group, note, hidden, isSystem), so
// the stored category goes out whole with the changes laid over it.
export const updateCategory = createAsyncThunk<void, { category: Category; changes: CategoryChanges }>(
  'categories/updateCategory',
  async ({ category, changes }) => {
    const row = { ...category, note: category.note ?? '', ...changes };
    // the monthly figures and goal are derived; the endpoint ignores them
    delete row.activity;
    delete row.balance;
    delete row.goal;
    await apiClient.put<unknown, Partial<Category>>(`categories/${category.id}`, {
      ...row,
      budgeted: undefined,
    });
  },
);

export const renameCategoryGroup = createAsyncThunk<void, { group: CategoryGroup; name: string }>(
  'categoryGroups/rename',
  async ({ group, name }) => {
    await apiClient.put(`category-groups/${group.id}`, {
      id: group.id,
      name,
      hidden: group.hidden ?? false,
      isSystem: group.isSystem,
    });
  },
);

type MonthlyKey = 'budgeted' | 'activity' | 'balance';

/** Adds (sign 1) or removes (sign -1) a category's monthly figures from its group's totals. */
function shiftGroupTotals(group: CategoryGroup, category: Category, sign: 1 | -1) {
  for (const key of ['budgeted', 'activity', 'balance'] as MonthlyKey[]) {
    const totals = { ...(group[key] ?? {}) };
    for (const [month, value] of Object.entries(category[key] ?? {})) {
      totals[month] = (totals[month] ?? 0) + sign * value;
    }
    group[key] = totals;
  }
}

/** Applies `update` to every copy of the category: the flat list and its group. */
function forEachCategoryCopy(state: CategoryState, categoryId: string, update: (cat: Category) => void) {
  state.allCategories.filter((c) => c.id === categoryId).forEach(update);
  for (const group of state.allCategoryGroups) {
    group.categories.filter((c) => c.id === categoryId).forEach(update);
  }
}

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
      .addCase(createCategory.fulfilled, (state, action) => {
        const { id, name, categoryGroupId } = action.payload;
        const { budgetId, note, hidden, budgeted } = action.meta.arg;

        const newCategory: Category = {
          id,
          name,
          categoryGroupId,
          budgetId,
          note,
          hidden,
          budgeted: budgeted ?? {},
          activity: {},
          balance: {},
        };

        state.allCategories.push(newCategory);

        const group = state.allCategoryGroups.find((g) => g.id === categoryGroupId);
        if (group) {
          group.categories.push(newCategory);
        }

        state.error = null;
      })
      .addCase(createCategory.rejected, (state, action) => {
        state.error = action.error.message ?? 'Failed to create category';
      })
      .addCase(updateCategoryBudget.fulfilled, (state, action) => {
        const { budgeted, categoryId, month } = action.meta.arg;
        state.loading = LoadingState.SUCCESS;

        // Assigning money changes what is budgeted and available, never activity
        // (that is spending). The group's totals move by the same delta so the
        // group row agrees with its categories without a refetch.
        for (const group of state.allCategoryGroups) {
          const cat = group.categories.find((c) => c.id === categoryId);
          if (!cat) continue;

          const delta = budgeted - (cat.budgeted?.[month] ?? 0);
          cat.budgeted = { ...cat.budgeted, [month]: budgeted };
          cat.balance = { ...cat.balance, [month]: (cat.balance?.[month] ?? 0) + delta };
          group.budgeted = { ...group.budgeted, [month]: (group.budgeted?.[month] ?? 0) + delta };
          group.balance = { ...group.balance, [month]: (group.balance?.[month] ?? 0) + delta };
          break;
        }
        state.error = null;
      })
      .addCase(updateCategoryDetails.fulfilled, (state, action) => {
        const { categoryId } = action.meta.arg;
        // the server returns the trimmed values it stored
        const { note, description } = action.payload;
        const apply = (cat: Category) => {
          if (note !== undefined && note !== null) cat.note = note;
          if (description !== undefined && description !== null) cat.description = description;
        };
        state.allCategories.filter((c) => c.id === categoryId).forEach(apply);
        for (const group of state.allCategoryGroups) {
          group.categories.filter((c) => c.id === categoryId).forEach(apply);
        }
      })
      .addCase(updateCategory.fulfilled, (state, action) => {
        const { category, changes } = action.meta.arg;
        const id = category.id ?? '';
        state.allCategories.filter((c) => c.id === id).forEach((c) => Object.assign(c, changes));

        const from = state.allCategoryGroups.find((g) => g.categories.some((c) => c.id === id));
        const current = from?.categories.find((c) => c.id === id);
        if (!from || !current) return;
        Object.assign(current, changes);

        // Hiding or moving changes which group lists the category; carry its
        // figures across so both group rows stay right without a refetch.
        const targetId = current.hidden ? HIDDEN_GROUP_ID : current.categoryGroupId;
        if (from.id === targetId) return;
        let target = state.allCategoryGroups.find((g) => g.id === targetId);
        if (!target && targetId === HIDDEN_GROUP_ID) {
          target = {
            id: HIDDEN_GROUP_ID,
            name: 'Hidden',
            hidden: true,
            collapsed: true,
            isSystem: false,
            budgeted: {},
            activity: {},
            balance: {},
            categories: [],
          };
          state.allCategoryGroups.push(target);
        }
        if (!target) return;
        from.categories = from.categories.filter((c) => c.id !== id);
        shiftGroupTotals(from, current, -1);
        target.categories.push(current);
        shiftGroupTotals(target, current, 1);
      })
      .addCase(renameCategoryGroup.fulfilled, (state, action) => {
        const group = state.allCategoryGroups.find((g) => g.id === action.meta.arg.group.id);
        if (group) group.name = action.meta.arg.name;
      })
      .addCase(updateCategoryGoal.fulfilled, (state, action) => {
        forEachCategoryCopy(state, action.meta.arg.categoryId, (cat) => {
          cat.goal = action.payload;
        });
      })
      .addCase(deleteCategoryGoal.fulfilled, (state, action) => {
        forEachCategoryCopy(state, action.meta.arg, (cat) => {
          cat.goal = null;
        });
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
