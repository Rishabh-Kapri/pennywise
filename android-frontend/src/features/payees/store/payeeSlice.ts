import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { LoadingState } from '../../../utils/constants';
import type { Payee, PayeeState } from '../types';

const initialState: PayeeState = {
  allPayees: [],
  loading: LoadingState.IDLE,
  error: null
};

export const fetchAllPayees = createAsyncThunk<Payee[]>('payees/fetchAllPayees', async () => {
  return apiClient.get<Payee[]>('payees');
});

export const createPayee = createAsyncThunk<Payee, Partial<Payee>>('payees/createPayee', async (payee) => {
  return apiClient.post<Payee>('payees', payee);
});

const payeeSlice = createSlice({
  name: 'payees',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllPayees.pending, (state) => {
        state.loading = LoadingState.PENDING;
      })
      .addCase(fetchAllPayees.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.allPayees = action.payload ?? [];
      })
      .addCase(fetchAllPayees.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load payees';
      })
      .addCase(createPayee.fulfilled, (state, action) => {
        state.allPayees.unshift(action.payload);
      });
  }
});

export default payeeSlice.reducer;
