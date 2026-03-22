import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import type { LoanMetadata, LoanState } from '../types/loan.types';
import { apiClient, LoadingState } from '@/utils';
import type { RootState } from '@/app';

// Async thunks for API calls
export const fetchAllLoanMetadata = createAsyncThunk<LoanMetadata[]>(
  'loans/fetchAllLoanMetadata',
  async () => {
    const data = await apiClient.get<LoanMetadata[] | null>('loan-metadata');
    return data ?? [];
  },
);

export const createLoanMetadata = createAsyncThunk<LoanMetadata, LoanMetadata>(
  'loans/createLoanMetadata',
  async (loan) => {
    return await apiClient.post<LoanMetadata>('loan-metadata', loan);
  },
);

export const updateLoanMetadata = createAsyncThunk<
  LoanMetadata,
  { accountId: string; updates: Partial<LoanMetadata> }
>('loans/updateLoanMetadata', async ({ accountId, updates }) => {
  return await apiClient.patch<LoanMetadata>(`loan-metadata/${accountId}`, updates);
});

export const deleteLoanMetadata = createAsyncThunk<string, string>(
  'loans/deleteLoanMetadata',
  async (accountId) => {
    await apiClient.delete(`loan-metadata/${accountId}`);
    return accountId;
  },
);

function buildMetadataMap(loans: LoanMetadata[]): Record<string, LoanMetadata> {
  const map: Record<string, LoanMetadata> = {};
  for (const loan of loans) {
    map[loan.accountId] = loan;
  }
  return map;
}

const initialState: LoanState = {
  loanMetadata: {},
  loading: LoadingState.IDLE,
  error: null,
};

const loanSlice = createSlice({
  name: 'loans',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      // fetchAll
      .addCase(fetchAllLoanMetadata.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchAllLoanMetadata.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.loanMetadata = buildMetadataMap(action.payload);
        state.error = null;
      })
      .addCase(fetchAllLoanMetadata.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? null;
      })
      // create
      .addCase(createLoanMetadata.fulfilled, (state, action) => {
        state.loanMetadata[action.payload.accountId] = action.payload;
      })
      .addCase(createLoanMetadata.rejected, (state, action) => {
        state.error = action.error.message ?? null;
      })
      // update
      .addCase(updateLoanMetadata.fulfilled, (state, action) => {
        state.loanMetadata[action.payload.accountId] = action.payload;
      })
      .addCase(updateLoanMetadata.rejected, (state, action) => {
        state.error = action.error.message ?? null;
      })
      // delete
      .addCase(deleteLoanMetadata.fulfilled, (state, action) => {
        delete state.loanMetadata[action.payload];
      })
      .addCase(deleteLoanMetadata.rejected, (state, action) => {
        state.error = action.error.message ?? null;
      });
  },
});

export default loanSlice.reducer;

// Selectors
export const selectLoanByAccountId = (accountId: string) => (state: RootState) =>
  state.loans.loanMetadata[accountId] ?? null;

export const selectAllLoanMetadata = (state: RootState) =>
  state.loans.loanMetadata;

export const selectLoanLoading = (state: RootState) => state.loans.loading;
