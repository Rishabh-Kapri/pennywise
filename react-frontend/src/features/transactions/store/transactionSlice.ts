import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import type {
  Transaction,
  TransactionDTO,
  TransactionLocationDTO,
  TransactionState,
  TransactionStatusDTO,
} from '../types/transaction.types';
import { TransactionStatus } from '../types/transaction.types';
import { apiClient, LoadingState } from '@/utils';
import { type PaginationResponse } from '@/utils/common.constants';

const initialState: TransactionState = {
  transactions: [],
  optimisticTransactions: {},
  loading: LoadingState.IDLE,
  loadingMore: LoadingState.IDLE,
  nextCursor: null,
  total: 0,
  pendingApprovalCount: 0,
  error: null,
};

export type FetchTransactionArgs = {
  accountIds?: string[];
  categoryIds?: string[];
  payeeIds?: string[];
  tagIds?: string[];
  note?: string;
  startDate?: string;
  endDate?: string;
  status?: TransactionStatus;
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
    // The edit was made on a copy that may be older than the row; the row's
    // version is the last one the server confirmed.
    version: transaction.version,
  });
}

/** The part of the root state these thunks read; the store imports this file. */
type TransactionsRoot = { transactions: TransactionState };

/** The server version of a loaded transaction, sent as If-Match. */
function storedVersion(state: TransactionsRoot, id: string): number | undefined {
  return state.transactions.transactions.find((txn) => txn.id === id)?.version;
}

const writeQueues = new Map<string, Promise<unknown>>();

/**
 * Runs one transaction's writes one after another. Each write sends the
 * version the previous one returned; two in flight together (an autosave on
 * blur, then another) would both send the old version and the second would
 * be refused as a conflict.
 */
function queueWrite<T>(id: string, write: () => Promise<T>): Promise<T> {
  const previous = writeQueues.get(id) ?? Promise.resolve();
  const next = previous.catch(() => undefined).then(write);
  writeQueues.set(id, next);
  next
    .finally(() => {
      if (writeQueues.get(id) === next) writeQueues.delete(id);
    })
    .catch(() => undefined);
  return next;
}

/** The server refused a write because the transaction changed after it was loaded. */
export function isVersionConflict(error: unknown): boolean {
  return (
    !!error && typeof error === 'object' && 'code' in error && error.code === 'TRANSACTION_VERSION_CONFLICT'
  );
}

/** The query string of `transactions/normalized` for these filters. */
export function transactionListQuery(args: FetchTransactionArgs = {}): string {
  const params = new URLSearchParams();

  if (args.accountIds && args.accountIds.length > 0) {
    params.set('accountId[]', args.accountIds.join(','));
  }
  if (args.categoryIds && args.categoryIds.length > 0) {
    params.set('categoryId[]', args.categoryIds.join(','));
  }
  if (args.payeeIds && args.payeeIds.length > 0) {
    params.set('payeeId[]', args.payeeIds.join(','));
  }
  if (args.tagIds && args.tagIds.length > 0) {
    params.set('tagId[]', args.tagIds.join(','));
  }
  if (args.note && args.note.trim()) {
    params.set('note', args.note);
  }
  if (args.startDate && args.startDate.trim()) {
    params.set('startDate', args.startDate);
  }
  if (args.endDate && args.endDate.trim()) {
    params.set('endDate', args.endDate);
  }
  if (args.status) {
    params.set('status', args.status);
  }
  if (args.cursor) {
    params.set('cursor', args.cursor);
  }
  params.set('limit', String(args.limit ?? 30));
  return params.toString();
}

export const fetchAllTransaction = createAsyncThunk<
  PaginationResponse<Transaction[]>,
  FetchTransactionArgs | undefined
>('transactions/fetchAllTransactions', async (args = {}) => {
  return await apiClient.get<PaginationResponse<Transaction[]>>(`transactions/normalized?${transactionListQuery(args)}`);
});

/** The server's count: the list is paginated, so the loaded rows can't give it. */
export const fetchPendingApprovalCount = createAsyncThunk<number, string | undefined>(
  'transactions/fetchPendingApprovalCount',
  async (accountId) => {
    const params = new URLSearchParams({ status: TransactionStatus.UNAPPROVED, limit: '1' });
    if (accountId) params.set('accountId[]', accountId);
    const res = await apiClient.get<PaginationResponse<Transaction[]>>(`transactions/normalized?${params}`);
    return res.total;
  },
);

export const deleteTransactionById = createAsyncThunk<Transaction[], string, { state: TransactionsRoot }>(
  'transactions/deleteTransaction',
  async (transactionId: string, { getState }) =>
    queueWrite(transactionId, () =>
      apiClient.delete<Transaction[]>(`transactions/${transactionId}`, {
        ifMatch: storedVersion(getState(), transactionId),
      }),
    ),
);

export const bulkDeleteTransactions = createAsyncThunk<{ deleted: number }, string[]>(
  'transactions/bulkDeleteTransactions',
  async (ids) => apiClient.post<{ deleted: number }, { ids: string[] }>('transactions/bulk-delete', { ids }),
);

export const createTransaction = createAsyncThunk<TransactionDTO, TransactionDTO>(
  'transactions/createTransaction',
  async (transaction: TransactionDTO) => {
    const url = `transactions`;
    return await apiClient.post(url, transaction);
  },
);

export const updateTransaction = createAsyncThunk<
  TransactionDTO & { version?: number },
  UpdateTransactionArgs,
  { state: TransactionsRoot }
>('transactions/updateTransaction', async ({ payload }: UpdateTransactionArgs, { getState }) => {
  const url = `transactions/${payload.id}`;
  if (!payload.id) return await apiClient.patch(url, payload);
  const id = payload.id;
  return queueWrite(id, () => apiClient.patch(url, payload, { ifMatch: storedVersion(getState(), id) }));
});

type UpdateTransactionLocationArgs = {
  id: string;
  location: TransactionLocationDTO;
};

export const updateTransactionLocation = createAsyncThunk<
  Transaction,
  UpdateTransactionLocationArgs,
  { state: TransactionsRoot }
>('transactions/updateTransactionLocation', async ({ id, location }: UpdateTransactionLocationArgs, { getState }) =>
  queueWrite(id, () =>
    apiClient.patch<Transaction>(`transactions/${id}/location`, location, { ifMatch: storedVersion(getState(), id) }),
  ),
);

export const updateTransactionStatus = createAsyncThunk<
  TransactionStatusDTO & { version?: number },
  TransactionStatusDTO,
  { state: TransactionsRoot }
>('transactions/updateTransactionStatus', async ({ id, status }: TransactionStatusDTO, { getState }) => {
  const res = await queueWrite(id, () =>
    apiClient.patch<{ version?: number } | null>(
      `transactions/${id}/status`,
      { status },
      { ifMatch: storedVersion(getState(), id) },
    ),
  );
  return { id, status, version: res?.version };
});

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
      .addCase(fetchPendingApprovalCount.fulfilled, (state, action) => {
        state.pendingApprovalCount = action.payload;
      })
      .addCase(deleteTransactionById.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(deleteTransactionById.fulfilled, (state) => {
        state.loading = LoadingState.SUCCESS;
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
        const id = action.meta.arg.payload.id;
        if (!id) return;
        delete state.optimisticTransactions[id];
        const transaction = state.transactions.find((txn) => txn.id === id);
        if (transaction && typeof action.payload?.version === 'number') {
          transaction.version = action.payload.version;
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
          // An earlier save may have moved the row on since the snapshot.
          state.transactions[index] = { ...previous, version: state.transactions[index].version };
        }
        delete state.optimisticTransactions[id];
      })
      .addCase(updateTransactionLocation.fulfilled, (state, action) => {
        const transaction = state.transactions.find((txn) => txn.id === action.payload.id);
        if (transaction) {
          transaction.locationLat = action.payload.locationLat ?? null;
          transaction.locationLng = action.payload.locationLng ?? null;
          transaction.locationName = action.payload.locationName ?? null;
          transaction.locationSource = action.payload.locationSource ?? null;
          if (typeof action.payload.version === 'number') transaction.version = action.payload.version;
        }
      })
      .addCase(updateTransactionLocation.rejected, (state, action) => {
        state.error = action.error.message ?? 'Failed to update transaction location';
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
          if (typeof action.payload.version === 'number') transaction.version = action.payload.version;
        }
      })
      .addCase(updateTransactionStatus.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to update transaction status';
      });
  },
});

export default transactionSlice.reducer;
