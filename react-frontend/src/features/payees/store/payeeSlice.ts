import { apiClient, LoadingState } from '@/utils';
import type { Payee, PayeeState } from '../types/payee.types';
import { createAsyncThunk, createSlice } from '@reduxjs/toolkit';

const initialState: PayeeState = {
  allPayees: [],
  loading: LoadingState.IDLE,
  error: null,
};

export const fetchAllPayees = createAsyncThunk<Payee[]>(
  'payees/fetchAllPayees',
  async () => {
    const res = await apiClient.get<Payee[]>(`payees`);
    return res;
  },
);

const payeesSlice = createSlice({
  name: 'payees',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllPayees.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchAllPayees.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.allPayees = action.payload ?? [];
        state.error = null;
      })
      .addCase(fetchAllPayees.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load payees';
      });
  },
});

export default payeesSlice.reducer;
