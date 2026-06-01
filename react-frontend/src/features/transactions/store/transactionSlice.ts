import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import type { Transaction, TransactionDTO, TransactionState, TransactionStatusDTO } from '../types/transaction.types';
import { apiClient, LoadingState } from '@/utils';
import { type PaginationResponse } from '@/utils/common.constants';

const initialState: TransactionState = {
  transactions: [],
  optimisticTransactions: {},
  loading: LoadingState.IDLE,
  loadingMore: LoadingState.IDLE,
  nextCursor: null,
  total: 0,
  error: null,
};

type FetchTransactionArgs = {
  accountIds?: string[];
  cursor?: string;
  limit?: number;
};

type UpdateTransactionArgs = {
  payload: TransactionDTO;
  optimisticTransaction: Transaction;
};

function applyOptimisticTransaction(transaction: Transaction, optimisticTransaction: Transaction) {
  const status = optimisticTransaction.status ?? transaction.status;
  Object.assign(transaction, optimisticTransaction, {
    status,
    tagIds: [...(optimisticTransaction.tagIds ?? [])],
  });
}

export const fetchAllTransaction = createAsyncThunk<
  PaginationResponse<Transaction[]>,
  FetchTransactionArgs | undefined
>('transactions/fetchAllTransactions', async (args = {}) => {
  let url = `transactions/normalized`;
  const params = new URLSearchParams();
  
  if (args?.accountIds) {
    params.set('accountId[]', args.accountIds.join(','))
  }
  if (args?.cursor) {
    params.set('cursor', args.cursor)
  }
  params.set('limit', String(args?.limit ?? 30))

  const queryParams = params.toString()
  url = `${url}${queryParams ? `?${queryParams}` : ''}`

  return await apiClient.get<PaginationResponse<Transaction[]>>(url);
});

export const deleteTransactionById = createAsyncThunk<Transaction[], string>(
  'transactions/deleteTransaction',
  async (transactionId: string) => {
    const url = `transactions/${transactionId}`;
    return await apiClient.delete(url);
  },
);

export const createTransaction = createAsyncThunk<TransactionDTO, TransactionDTO>(
  'transactions/createTransaction',
  async (transaction: TransactionDTO) => {
    const url = `transactions`;
    return await apiClient.post(url, transaction);
  },
);

export const updateTransaction = createAsyncThunk<TransactionDTO, UpdateTransactionArgs>(
  'transactions/updateTransaction',
  async ({ payload }: UpdateTransactionArgs) => {
    const url = `transactions/${payload.id}`;
    return await apiClient.patch(url, payload);
  },
);

export const updateTransactionStatus = createAsyncThunk<TransactionStatusDTO, TransactionStatusDTO>(
  'transactions/updateTransactionStatus',
  async ({ id, status }: TransactionStatusDTO) => {
    const url = `transactions/${id}/status`;
    await apiClient.patch(url, { status });
    return { id, status };
  },
);

const transactionSlice = createSlice({
  name: 'transactions',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllTransaction.pending, (state, action) => {
        const isLoadingMore = Boolean(action.meta.arg?.cursor);

        if (isLoadingMore) {
          state.loadingMore = LoadingState.PENDING;
        } else {
          state.loading = LoadingState.PENDING;
          state.loadingMore = LoadingState.IDLE;
          state.transactions = [];
          state.nextCursor = null;
        }

        state.error = null;
      })
      .addCase(fetchAllTransaction.fulfilled, (state, action) => {
        const isLoadingMore = Boolean(action.meta.arg?.cursor);
        const data = action.payload.data ?? [];

        if (isLoadingMore) {
          state.transactions.push(...data);
          state.loadingMore = LoadingState.SUCCESS;
        } else {
          state.transactions = data;
          state.loading = LoadingState.SUCCESS;
          state.loadingMore = LoadingState.SUCCESS;
        }

        state.nextCursor = action.payload.pagination.nextCursor ?? '';
        state.total = action.payload.total;
        state.error = null;
      })
      .addCase(fetchAllTransaction.rejected, (state, action) => {
        const isLoadingMore = Boolean(action.meta.arg?.cursor);
        if (isLoadingMore) {
          state.loadingMore = LoadingState.ERROR;
        } else {
          state.loading = LoadingState.ERROR;
        }
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
      .addCase(updateTransaction.pending, (state, action) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
        const id = action.meta.arg.payload.id;
        if (!id) return;
        const transaction = state.transactions.find((txn) => txn.id === id);
        if (!transaction) return;
        state.optimisticTransactions[id] = { ...transaction, tagIds: [...(transaction.tagIds ?? [])] };
        applyOptimisticTransaction(transaction, action.meta.arg.optimisticTransaction);
      })
      .addCase(updateTransaction.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.error = null;
        if (action.meta.arg.payload.id) {
          delete state.optimisticTransactions[action.meta.arg.payload.id];
        }
      })
      .addCase(updateTransaction.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to update transaction';
        const id = action.meta.arg.payload.id;
        if (!id) return;
        const previous = state.optimisticTransactions[id];
        const index = state.transactions.findIndex((txn) => txn.id === id);
        if (previous && index !== -1) {
          state.transactions[index] = previous;
        }
        delete state.optimisticTransactions[id];
      })
      .addCase(updateTransactionStatus.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(updateTransactionStatus.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.error = null;
        const transaction = state.transactions.find((txn) => txn.id === action.payload.id);
        if (transaction) {
          transaction.status = action.payload.status;
        }
      })
      .addCase(updateTransactionStatus.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to update transaction status';
      });
  },
});

export default transactionSlice.reducer;
