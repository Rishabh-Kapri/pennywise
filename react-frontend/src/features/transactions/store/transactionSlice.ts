import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import type {
  Transaction,
  TransactionDTO,
  TransactionState,
} from '../types/transaction.types';
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
  'transactions/deleteTransaction',
  async (transactionId: string) => {
    const url = `transactions/${transactionId}`;
    return await apiClient.delete(url);
  },
);

export const createTransaction = createAsyncThunk<
  TransactionDTO,
  TransactionDTO
>('transactions/createTransaction', async (transaction: TransactionDTO) => {
  const url = `transactions`;
  return await apiClient.post(url, transaction);
});

export const updateTransaction = createAsyncThunk<
  TransactionDTO,
  TransactionDTO
>('transactions/updateTransaction', async (transaction: TransactionDTO) => {
  const url = `transactions/${transaction.id}`;
  return await apiClient.patch(url, transaction);
});

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
      .addCase(deleteTransactionById.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(deleteTransactionById.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        console.log('transaction deleted', state, action);
      })
      .addCase(createTransaction.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(createTransaction.fulfilled, (state) => {
        state.loading = LoadingState.SUCCESS;
        state.error = null;
      })
      .addCase(createTransaction.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to create transaction';
      })
      .addCase(updateTransaction.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(updateTransaction.fulfilled, (state) => {
        state.loading = LoadingState.SUCCESS;
        state.error = null;
      })
      .addCase(updateTransaction.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to update transaction';
      });
  },
});

export default transactionSlice.reducer;
