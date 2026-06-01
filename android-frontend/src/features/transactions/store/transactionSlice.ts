import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { LoadingState, type PaginationResponse } from '../../../utils/constants';
import type { Transaction, TransactionDTO, TransactionState, TransactionStatus } from '../types';

type FetchTransactionArgs = {
  accountIds?: string[];
  cursor?: string;
  limit?: number;
};

const initialState: TransactionState = {
  transactions: [],
  optimisticTransactions: {},
  loading: LoadingState.IDLE,
  loadingMore: LoadingState.IDLE,
  nextCursor: null,
  total: 0,
  error: null
};

export const fetchAllTransactions = createAsyncThunk<PaginationResponse<Transaction[]>, FetchTransactionArgs | undefined>(
  'transactions/fetchAllTransactions',
  async (args = {}) => {
    const params = new URLSearchParams();
    if (args.accountIds?.length) params.set('accountId[]', args.accountIds.join(','));
    if (args.cursor) params.set('cursor', args.cursor);
    params.set('limit', String(args.limit ?? 30));
    const query = params.toString();
    return apiClient.get<PaginationResponse<Transaction[]>>(`transactions/normalized${query ? `?${query}` : ''}`);
  }
);

export const createTransaction = createAsyncThunk<TransactionDTO, TransactionDTO>('transactions/createTransaction', async (txn) => {
  return apiClient.post<TransactionDTO>('transactions', txn);
});

export const updateTransaction = createAsyncThunk<TransactionDTO, TransactionDTO>('transactions/updateTransaction', async (txn) => {
  return apiClient.patch<TransactionDTO>(`transactions/${txn.id}`, txn);
});

export const deleteTransactionById = createAsyncThunk<void, string>('transactions/deleteTransactionById', async (id) => {
  await apiClient.delete(`transactions/${id}`);
});

export const updateTransactionStatus = createAsyncThunk<{ id: string; status: TransactionStatus }, { id: string; status: TransactionStatus }>(
  'transactions/updateTransactionStatus',
  async ({ id, status }) => {
    await apiClient.patch(`transactions/${id}/status`, { status });
    return { id, status };
  }
);

const transactionSlice = createSlice({
  name: 'transactions',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllTransactions.pending, (state, action) => {
        const loadingMore = Boolean(action.meta.arg?.cursor);
        if (loadingMore) state.loadingMore = LoadingState.PENDING;
        else {
          state.loading = LoadingState.PENDING;
          state.transactions = [];
          state.nextCursor = null;
        }
        state.error = null;
      })
      .addCase(fetchAllTransactions.fulfilled, (state, action) => {
        const loadingMore = Boolean(action.meta.arg?.cursor);
        const rows = action.payload.data ?? [];
        if (loadingMore) {
          state.transactions.push(...rows);
          state.loadingMore = LoadingState.SUCCESS;
        } else {
          state.transactions = rows;
          state.loading = LoadingState.SUCCESS;
        }
        state.nextCursor = action.payload.pagination.nextCursor ?? null;
        state.total = action.payload.total;
      })
      .addCase(fetchAllTransactions.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load transactions';
      })
      .addCase(updateTransactionStatus.fulfilled, (state, action) => {
        const txn = state.transactions.find((item) => item.id === action.payload.id);
        if (txn) txn.status = action.payload.status;
      });
  }
});

export default transactionSlice.reducer;
