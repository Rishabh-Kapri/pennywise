import { apiClient, LoadingState } from '@/utils';
import type { Payee, PayeeMatchType, PayeeMergeResult, PayeeRule, PayeeState, PayeeStats } from '../types/payee.types';
import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';

const initialState: PayeeState = {
  allPayees: [],
  loading: LoadingState.IDLE,
  error: null,
  statsById: {},
  rules: [],
  rulesLoading: LoadingState.IDLE,
  rulesError: null,
};

export const fetchAllPayees = createAsyncThunk<Payee[]>(
  'payees/fetchAllPayees',
  async () => {
    const res = await apiClient.get<Payee[]>(`payees`);
    return res;
  },
);

export const fetchPayeeStats = createAsyncThunk<PayeeStats[]>('payees/fetchPayeeStats', async () => {
  return (await apiClient.get<PayeeStats[]>('payees/stats')) ?? [];
});

export const fetchPayeeRules = createAsyncThunk<PayeeRule[]>('payees/fetchPayeeRules', async () => {
  return (await apiClient.get<PayeeRule[]>('payees/rules')) ?? [];
});

/** Payees, their usage and every rule: what the Payees screen shows. */
export const refreshPayeeData = createAsyncThunk('payees/refreshPayeeData', async (_, { dispatch }) => {
  await Promise.all([dispatch(fetchAllPayees()), dispatch(fetchPayeeStats()), dispatch(fetchPayeeRules())]);
});

export const createPayee = createAsyncThunk<Payee, string>('payees/createPayee', async (name, { dispatch }) => {
  const created = await apiClient.post<Payee>('payees', { name } as Partial<Payee>);
  await dispatch(refreshPayeeData());
  return created;
});

export const renamePayee = createAsyncThunk<void, { id: string; name: string }>(
  'payees/renamePayee',
  async ({ id, name }, { dispatch }) => {
    await apiClient.patch(`payees/${id}`, { name } as Partial<Payee>);
    await dispatch(fetchAllPayees());
  },
);

/** Deletes payees one by one; resolves with the ids that failed. */
export const deletePayees = createAsyncThunk<string[], string[]>('payees/deletePayees', async (ids, { dispatch }) => {
  const results = await Promise.allSettled(ids.map((id) => apiClient.delete(`payees/${id}`)));
  await dispatch(refreshPayeeData());
  return ids.filter((_, index) => results[index].status === 'rejected');
});

export const mergePayees = createAsyncThunk<PayeeMergeResult, { targetId: string; sourceIds: string[] }>(
  'payees/mergePayees',
  async (body, { dispatch }) => {
    const result = await apiClient.post<PayeeMergeResult>('payees/merge', body);
    await dispatch(refreshPayeeData());
    return result;
  },
);

export interface PayeeRuleInput {
  /** set when editing */
  ruleId?: string;
  /** the payee the rule belongs to now (editing: before any move) */
  originalPayeeId?: string;
  originalMatchString?: string;
  payeeId: string;
  matchString: string;
  matchType: PayeeMatchType;
  categoryId: string;
}

/**
 * Creates or edits a rule. Rules are unique per budget by match string and the
 * API upserts on it, so posting the same string under another payee moves the
 * rule there. A move that also changes the string leaves the old row behind,
 * which is deleted here.
 */
export const savePayeeRule = createAsyncThunk<void, PayeeRuleInput>(
  'payees/savePayeeRule',
  async (input, { dispatch }) => {
    const payload = {
      matchString: input.matchString,
      matchType: input.matchType,
      categoryId: input.categoryId || undefined,
    } as Partial<PayeeRule>;
    const isMove = Boolean(input.ruleId && input.originalPayeeId && input.originalPayeeId !== input.payeeId);

    if (input.ruleId && !isMove) {
      await apiClient.patch(`payees/${input.payeeId}/rules/${input.ruleId}`, payload);
    } else {
      await apiClient.post(`payees/${input.payeeId}/rules`, payload);
      if (isMove && input.originalMatchString !== input.matchString) {
        await apiClient.delete(`payees/${input.originalPayeeId}/rules/${input.ruleId}`);
      }
    }
    await Promise.all([dispatch(fetchPayeeRules()), dispatch(fetchPayeeStats())]);
  },
);

export const deletePayeeRule = createAsyncThunk<void, { payeeId: string; ruleId: string }>(
  'payees/deletePayeeRule',
  async ({ payeeId, ruleId }, { dispatch }) => {
    await apiClient.delete(`payees/${payeeId}/rules/${ruleId}`);
    await Promise.all([dispatch(fetchPayeeRules()), dispatch(fetchPayeeStats())]);
  },
);

const payeesSlice = createSlice({
  name: 'payees',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllPayees.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchAllPayees.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.allPayees = action.payload ?? [];
        state.error = null;
      })
      .addCase(fetchAllPayees.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load payees';
      })
      .addCase(fetchPayeeStats.fulfilled, (state, action) => {
        state.statsById = Object.fromEntries(action.payload.map((stats) => [stats.payeeId, stats]));
      })
      .addCase(fetchPayeeRules.pending, (state) => {
        state.rulesLoading = LoadingState.PENDING;
        state.rulesError = null;
      })
      .addCase(fetchPayeeRules.fulfilled, (state, action) => {
        state.rulesLoading = LoadingState.SUCCESS;
        state.rules = action.payload;
      })
      .addCase(fetchPayeeRules.rejected, (state, action) => {
        state.rulesLoading = LoadingState.ERROR;
        state.rulesError = action.error.message ?? 'Failed to load payee rules';
      });
  },
});

export default payeesSlice.reducer;
