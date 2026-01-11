import {
  createAsyncThunk,
  createSelector,
  createSlice,
  type PayloadAction,
} from '@reduxjs/toolkit';
import type { Budget, BudgetState } from '../types/budget.types';
import { LoadingState, apiClient, getCurrentMonthKey } from '@/utils';
import type { RootState } from '@/app/store';
import { getSelectedMonthInHumanFormat } from '@/utils/date.utils';

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

const budgetSlice = createSlice({
  name: 'budgets',
  initialState,
  reducers: {
    setSelectedMonth: (state, action: PayloadAction<string>) => {
      state.selectedMonth = action.payload;
    },
    setSelectedBudget: (state, action: PayloadAction<Budget>) => {
      state.selectedBudget = action.payload;
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
        state.allBudgets = action.payload;
        if (!state.selectedBudget) {
          state.selectedBudget =
            action.payload.find((budget) => budget.isSelected) ?? null;
        }
        state.selectedMonth = getCurrentMonthKey();
        state.error = null;
      })
      .addCase(fetchAllBudgets.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load budgets';
      });
  },
});

export const { setSelectedMonth, setSelectedBudget } = budgetSlice.actions;

export default budgetSlice.reducer;

// Selectors
// const selectBudgetState = (state: RootState) => state.budget;
// const selectAllBudget = (state: RootState) => state.budget.allBudgets;
export const selectSelectedMonth = (state: RootState) => state.budgets.selectedMonth;
export const selectSelectedBudget = (state: RootState) => state.budgets.selectedBudget;

export const selectMonthInHumanFormat = createSelector(
  [selectSelectedMonth],
  (month) => getSelectedMonthInHumanFormat(month),
);
