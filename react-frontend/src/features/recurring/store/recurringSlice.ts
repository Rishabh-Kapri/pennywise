import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import { apiClient, LoadingState } from '@/utils';
import type { RootState } from '@/app';
import type {
  RecurringRunResult,
  RecurringState,
  RecurringTransaction,
  RecurringTransactionDraft,
} from '../types/recurring.types';

const ENDPOINT = 'recurring-transactions';

export const fetchRecurringTransactions = createAsyncThunk<RecurringTransaction[]>(
  'recurring/fetchAll',
  async () => {
    const data = await apiClient.get<RecurringTransaction[] | null>(ENDPOINT);
    return data ?? [];
  },
);

export const createRecurringTransaction = createAsyncThunk<
  RecurringTransaction,
  RecurringTransactionDraft
>('recurring/create', async (draft) => {
  return await apiClient.post<RecurringTransaction>(ENDPOINT, draft);
});

export const updateRecurringTransaction = createAsyncThunk<
  RecurringTransaction,
  { id: string; draft: RecurringTransactionDraft }
>('recurring/update', async ({ id, draft }) => {
  return await apiClient.patch<RecurringTransaction>(`${ENDPOINT}/${id}`, draft);
});

export const deleteRecurringTransaction = createAsyncThunk<string, string>(
  'recurring/delete',
  async (id) => {
    await apiClient.delete(`${ENDPOINT}/${id}`);
    return id;
  },
);

export const runRecurringTransactions = createAsyncThunk<RecurringRunResult, void>(
  'recurring/run',
  async (_, { dispatch }) => {
    const result = await apiClient.post<RecurringRunResult>(`${ENDPOINT}/run`, {});
    // rules advance their nextDate when they fire, so pull the fresh list
    await dispatch(fetchRecurringTransactions());
    return result;
  },
);

const initialState: RecurringState = {
  rules: [],
  loading: LoadingState.IDLE,
  saving: false,
  lastRun: null,
  error: null,
};

const recurringSlice = createSlice({
  name: 'recurring',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchRecurringTransactions.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchRecurringTransactions.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.rules = action.payload;
      })
      .addCase(fetchRecurringTransactions.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load recurring transactions';
      })
      .addCase(createRecurringTransaction.pending, (state) => {
        state.saving = true;
        state.error = null;
      })
      .addCase(createRecurringTransaction.fulfilled, (state, action) => {
        state.saving = false;
        state.rules.push(action.payload);
      })
      .addCase(createRecurringTransaction.rejected, (state, action) => {
        state.saving = false;
        state.error = action.error.message ?? 'Failed to create recurring transaction';
      })
      .addCase(updateRecurringTransaction.pending, (state) => {
        state.saving = true;
        state.error = null;
      })
      .addCase(updateRecurringTransaction.fulfilled, (state, action) => {
        state.saving = false;
        const index = state.rules.findIndex((rule) => rule.id === action.payload.id);
        if (index >= 0) {
          state.rules[index] = action.payload;
        }
      })
      .addCase(updateRecurringTransaction.rejected, (state, action) => {
        state.saving = false;
        state.error = action.error.message ?? 'Failed to update recurring transaction';
      })
      .addCase(deleteRecurringTransaction.fulfilled, (state, action) => {
        state.rules = state.rules.filter((rule) => rule.id !== action.payload);
      })
      .addCase(deleteRecurringTransaction.rejected, (state, action) => {
        state.error = action.error.message ?? 'Failed to delete recurring transaction';
      })
      .addCase(runRecurringTransactions.fulfilled, (state, action) => {
        state.lastRun = action.payload;
      })
      .addCase(runRecurringTransactions.rejected, (state, action) => {
        state.error = action.error.message ?? 'Failed to run recurring transactions';
      });
  },
});

export default recurringSlice.reducer;

// Selectors
export const selectRecurringRules = (state: RootState) => state.recurring.rules;
export const selectRecurringLoading = (state: RootState) => state.recurring.loading;
export const selectRecurringSaving = (state: RootState) => state.recurring.saving;
export const selectRecurringError = (state: RootState) => state.recurring.error;
export const selectRecurringLastRun = (state: RootState) => state.recurring.lastRun;
