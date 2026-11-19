import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { LoadingState, type PaginationResponse } from '../../../utils/constants';
import type { Transaction, TransactionDTO, TransactionLocationDTO, TransactionState, TransactionStatus } from '../types';

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

/**
 * The newest version the server has told this app about, per transaction.
 * Screens keep their own copies of a list, so after one write (a location pin,
 * say) a screen can still hold the older version; the newer one wins.
 */
const latestVersions = new Map<string, number>();

function rememberVersion(id: string, version: unknown) {
  if (typeof version !== 'number') return;
  latestVersions.set(id, Math.max(version, latestVersions.get(id) ?? 0));
}

/** The version to send as If-Match; undefined when the app doesn't know one. */
function versionFor(id: string, given?: number): number | undefined {
  const known = latestVersions.get(id);
  if (given === undefined) return known;
  return known !== undefined && known > given ? known : given;
}

const writeQueues = new Map<string, Promise<unknown>>();

/**
 * Runs one transaction's writes one after another, so each sends the version
 * the previous one returned. Two in flight together would both send the old
 * version and the second would be refused as a conflict.
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
  return !!error && typeof error === 'object' && 'code' in error && error.code === 'TRANSACTION_VERSION_CONFLICT';
}

export const updateTransaction = createAsyncThunk<TransactionDTO, TransactionDTO>('transactions/updateTransaction', async (txn) => {
  const { version, ...body } = txn;
  if (!txn.id) return apiClient.patch<TransactionDTO>(`transactions/${txn.id}`, body);
  const id = txn.id;
  const saved = await queueWrite(id, () =>
    apiClient.patch<TransactionDTO>(`transactions/${id}`, body, { ifMatch: versionFor(id, version) })
  );
  rememberVersion(id, saved?.version);
  return saved;
});

export const deleteTransactionById = createAsyncThunk<void, { id: string; version?: number }>(
  'transactions/deleteTransactionById',
  async ({ id, version }) => {
    await queueWrite(id, () => apiClient.delete(`transactions/${id}`, { ifMatch: versionFor(id, version) }));
    latestVersions.delete(id);
  }
);

export const updateTransactionLocation = createAsyncThunk<
  Transaction,
  { id: string; location: TransactionLocationDTO; version?: number }
>('transactions/updateTransactionLocation', async ({ id, location, version }) => {
  const saved = await queueWrite(id, () =>
    apiClient.patch<Transaction>(`transactions/${id}/location`, location, { ifMatch: versionFor(id, version) })
  );
  rememberVersion(id, saved?.version);
  return saved;
});

export const updateTransactionStatus = createAsyncThunk<
  { id: string; status: TransactionStatus; version?: number },
  { id: string; status: TransactionStatus; version?: number }
>('transactions/updateTransactionStatus', async ({ id, status, version }) => {
  const res = await queueWrite(id, () =>
    apiClient.patch<{ version?: number } | null>(
      `transactions/${id}/status`,
      { status },
      { ifMatch: versionFor(id, version) }
    )
  );
  rememberVersion(id, res?.version);
  return { id, status, version: res?.version };
});

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
        if (txn) {
          txn.status = action.payload.status;
          if (typeof action.payload.version === 'number') txn.version = action.payload.version;
        }
      })
      .addCase(updateTransactionLocation.fulfilled, (state, action) => {
        const txn = state.transactions.find((item) => item.id === action.payload.id);
        if (txn) {
          txn.locationLat = action.payload.locationLat ?? null;
          txn.locationLng = action.payload.locationLng ?? null;
          txn.locationName = action.payload.locationName ?? null;
          txn.locationSource = action.payload.locationSource ?? null;
          if (typeof action.payload.version === 'number') txn.version = action.payload.version;
        }
      });
  }
});

export default transactionSlice.reducer;
