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
      });
  }
});

export default loanSlice.reducer;
