import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { LoadingState } from '../../../utils/constants';
import type { LoanMetadata, LoanState } from '../types';

const initialState: LoanState = {
  loanMetadata: {},
  loading: LoadingState.IDLE,
  error: null
};

export const fetchAllLoanMetadata = createAsyncThunk<LoanMetadata[]>('loans/fetchAllLoanMetadata', async () => {
  const data = await apiClient.get<LoanMetadata[] | null>('loan-metadata');
  return data ?? [];
});

export const updateLoanMetadata = createAsyncThunk<LoanMetadata, { accountId: string; updates: Partial<LoanMetadata> }>(
  'loans/updateLoanMetadata',
  async ({ accountId, updates }) => apiClient.patch<LoanMetadata>(`loan-metadata/${accountId}`, updates)
);

const loanSlice = createSlice({
  name: 'loans',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllLoanMetadata.pending, (state) => {
        state.loading = LoadingState.PENDING;
      })
      .addCase(fetchAllLoanMetadata.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.loanMetadata = Object.fromEntries(action.payload.map((item) => [item.accountId, item]));
      })
      .addCase(fetchAllLoanMetadata.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load loan metadata';
      })
      .addCase(updateLoanMetadata.fulfilled, (state, action) => {
        state.loanMetadata[action.payload.accountId] = action.payload;
      });
  }
});

export default loanSlice.reducer;
