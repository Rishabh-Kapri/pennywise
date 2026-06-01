import { createAsyncThunk, createSelector, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { getCurrentMonthKey, getSelectedMonthInHumanFormat } from '../../../utils/date';
import { LoadingState } from '../../../utils/constants';
import type { RootState } from '../../../app/store';
import type { Budget, BudgetState, CreateBudgetPayload } from '../types';

const initialState: BudgetState = {
  allBudgets: [],
  selectedBudget: null,
  selectedMonth: '',
  loading: LoadingState.IDLE,
  error: null
};

export const fetchAllBudgets = createAsyncThunk<Budget[]>('budgets/fetchAllBudgets', async () => {
  return apiClient.get<Budget[]>('budgets');
});

export const createBudget = createAsyncThunk<Budget, CreateBudgetPayload>('budgets/createBudget', async (budget) => {
  return apiClient.post<Budget>('budgets', budget);
});

export const updateBudgetSelection = createAsyncThunk<void, { budget: Budget; isSelected: boolean }>(
  'budgets/updateBudgetSelection',
  async ({ budget, isSelected }) => {
    if (!budget.id) return;
    await apiClient.patch<Budget>(`budgets/${budget.id}`, { ...budget, isSelected });
  }
);

const budgetSlice = createSlice({
  name: 'budgets',
  initialState,
  reducers: {
    setSelectedMonth: (state, action: PayloadAction<string>) => {
      state.selectedMonth = action.payload;
    },
    setSelectedBudget: (state, action: PayloadAction<Budget>) => {
      state.selectedBudget = { ...action.payload, isSelected: true };
      state.allBudgets = state.allBudgets.map((budget) => ({
        ...budget,
        isSelected: budget.id === action.payload.id
      }));
    }
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
        state.selectedBudget = state.selectedBudget ?? action.payload.find((budget) => budget.isSelected) ?? action.payload[0] ?? null;
        state.selectedMonth = state.selectedMonth || getCurrentMonthKey();
      })
      .addCase(fetchAllBudgets.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load budgets';
      })
      .addCase(createBudget.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.selectedBudget = action.payload;
        state.allBudgets = [action.payload, ...state.allBudgets.filter((budget) => budget.id !== action.payload.id)];
        state.selectedMonth = getCurrentMonthKey();
      });
  }
});

export const { setSelectedBudget, setSelectedMonth } = budgetSlice.actions;
export default budgetSlice.reducer;

export const selectSelectedBudget = (state: RootState) => state.budgets.selectedBudget;
export const selectSelectedMonth = (state: RootState) => state.budgets.selectedMonth;
export const selectAllBudgets = (state: RootState) => state.budgets.allBudgets;
export const selectMonthInHumanFormat = createSelector([selectSelectedMonth], getSelectedMonthInHumanFormat);
