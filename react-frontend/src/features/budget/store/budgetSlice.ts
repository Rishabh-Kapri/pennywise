import {
  createAsyncThunk,
  createSelector,
  createSlice,
  type PayloadAction,
} from '@reduxjs/toolkit';
import type { Budget, BudgetState } from '../types/budget.types';
import type { BudgetOnboarding } from '@/features/onboarding/types';
import { LoadingState, apiClient, getCurrentMonthKey } from '@/utils';
import type { RootState } from '@/app/store';
import { getSelectedMonthInHumanFormat } from '@/utils/date.utils';

export interface BudgetTemplateGroupInput {
  name: string;
  categories: { name: string }[];
}

export interface CreateBudgetPayload {
  name: string;
  templateGroups: BudgetTemplateGroupInput[];
}

export interface UpdateBudgetSelectionPayload {
  budget: Budget;
  isSelected: boolean;
}

const initialState: BudgetState = {
  allBudgets: [],
  selectedBudget: null,
  selectedMonth: '',
  loading: 'idle',
  error: null,
};

export const fetchAllBudgets = createAsyncThunk<Budget[]>(
  'budgets/fetchAllBudgets',
  async () => {
    return await apiClient.get<Budget[]>('budgets');
  },
);

export const createBudget = createAsyncThunk<Budget, CreateBudgetPayload>(
  'budgets/createBudget',
  async (budget) => {
    return await apiClient.post<Budget>(
      'budgets',
      budget as unknown as Partial<Budget>,
    );
  },
);

export const updateBudgetSelection = createAsyncThunk<
  void,
  UpdateBudgetSelectionPayload
>('budgets/updateBudgetSelection', async ({ budget, isSelected }) => {
  // Selection is per member, and selecting one budget unselects the others,
  // so only a select is sent. Sending the name too would read as a rename,
  // which only the owner may do.
  if (!budget.id || !isSelected) {
    return;
  }

  await apiClient.patch<Budget, { isSelected: boolean }>(`budgets/${budget.id}`, { isSelected });
});

/** Reloads the budget list and opens budgetId, e.g. one just joined. */
export const openBudget = createAsyncThunk<void, string, { state: RootState }>(
  'budgets/openBudget',
  async (budgetId, { dispatch, getState }) => {
    await dispatch(fetchAllBudgets()).unwrap();
    const budget = getState().budgets.allBudgets.find((b) => b.id === budgetId);
    if (!budget) return;
    dispatch(setSelectedBudget(budget));
    await dispatch(updateBudgetSelection({ budget, isSelected: true }));
  },
);

const budgetSlice = createSlice({
  name: 'budgets',
  initialState,
  reducers: {
    setSelectedMonth: (state, action: PayloadAction<string>) => {
      state.selectedMonth = action.payload;
    },
    /** Stores the onboarding state the API returned for the selected budget. */
    budgetOnboardingUpdated: (state, action: PayloadAction<BudgetOnboarding>) => {
      if (!state.selectedBudget) return;
      state.selectedBudget.onboarding = action.payload;
      const budget = state.allBudgets.find((b) => b.id === state.selectedBudget?.id);
      if (budget) budget.onboarding = action.payload;
    },
    setSelectedBudget: (state, action: PayloadAction<Budget>) => {
      state.selectedBudget = { ...action.payload, isSelected: true };
      state.allBudgets = state.allBudgets.map((budget) => ({
        ...budget,
        isSelected: budget.id === action.payload.id,
      }));
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllBudgets.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchAllBudgets.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        const budgets = action.payload ?? [];
        state.allBudgets = budgets;
        // After switching accounts the old selection isn't in this user's list.
        const selectedId = state.selectedBudget?.id;
        if (!budgets.some((budget) => budget.id === selectedId)) {
          state.selectedBudget = budgets.find((budget) => budget.isSelected) ?? budgets[0] ?? null;
        }
        state.selectedMonth = getCurrentMonthKey();
        state.error = null;
      })
      .addCase(fetchAllBudgets.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load budgets';
      })
      .addCase(createBudget.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(createBudget.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.selectedBudget = action.payload;
        state.allBudgets = [
          action.payload,
          ...state.allBudgets.filter((budget) => budget.id !== action.payload.id),
        ];
        state.selectedMonth = getCurrentMonthKey();
        state.error = null;
      })
      .addCase(createBudget.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to create budget';
      });
  },
});

export const { setSelectedMonth, setSelectedBudget, budgetOnboardingUpdated } = budgetSlice.actions;

export default budgetSlice.reducer;

// Selectors
// const selectBudgetState = (state: RootState) => state.budget;
// const selectAllBudget = (state: RootState) => state.budget.allBudgets;
export const selectSelectedMonth = (state: RootState) => state.budgets.selectedMonth;
export const selectSelectedBudget = (state: RootState) => state.budgets.selectedBudget;
export const selectAllBudgets = (state: RootState) => state.budgets.allBudgets;
export const selectBudgetLoading = (state: RootState) => state.budgets.loading;
export const selectBudgetError = (state: RootState) => state.budgets.error;

export const selectMonthInHumanFormat = createSelector(
  [selectSelectedMonth],
  (month) => getSelectedMonthInHumanFormat(month),
);
