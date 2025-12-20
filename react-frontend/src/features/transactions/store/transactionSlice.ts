import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import type { Transaction, TransactionState } from '../types/transaction.types';
import { apiClient, LoadingState } from '@/utils';

const initialState: TransactionState = {
  transactions: [],
  loading: LoadingState.IDLE,
  error: null,
};

export const fetchAllTransaction = createAsyncThunk<Transaction[], string>(
  'transactions/fetchAllTransactions',
  async (accountId: string = '') => {
    let url = `transactions/normalized`;
    if (accountId) {
      url = `transactions/normalized?accountId=${accountId}`;
    }
    return await apiClient.get<Transaction[]>(url);
  },
);

export const deleteTransactionById = createAsyncThunk<Transaction[], string>(
  'transaction/deleteTransaction',
  async (transactionId: string) => {
    const url = `transactions/${transactionId}`;
    return await apiClient.delete(url);
  },
);

const transactionSlice = createSlice({
  name: 'transactions',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllTransaction.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchAllTransaction.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.transactions = action.payload ?? [];
        state.error = null;
      })
      .addCase(fetchAllTransaction.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load transactions';
      })
      .addCase(deleteTransactionById.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        console.log('transaction deleted', state, action);
      });
  },
});

export default transactionSlice.reducer;
