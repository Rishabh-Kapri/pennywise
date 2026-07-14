import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type {
  IncomeExpenseReport,
  NetWorthReport,
  ReportPreset,
  ReportRange,
  ReportState,
  SpendingReport,
} from '../types/report.types';
import { apiClient, LoadingState } from '@/utils';
import { addMonths, getCurrentMonthKey } from '@/utils/date.utils';
import type { RootState } from '@/app';

function rangeQuery({ startMonth, endMonth }: ReportRange): string {
  return `startMonth=${startMonth}&endMonth=${endMonth}`;
}

export const fetchSpendingReport = createAsyncThunk<SpendingReport, ReportRange>(
  'reports/fetchSpending',
  async (range) => {
    return await apiClient.get<SpendingReport>(`reports/spending?${rangeQuery(range)}`);
  },
);

export const fetchIncomeExpenseReport = createAsyncThunk<IncomeExpenseReport, ReportRange>(
  'reports/fetchIncomeExpense',
  async (range) => {
    return await apiClient.get<IncomeExpenseReport>(`reports/income-expense?${rangeQuery(range)}`);
  },
);

export const fetchNetWorthReport = createAsyncThunk<NetWorthReport, ReportRange>(
  'reports/fetchNetWorth',
  async (range) => {
    return await apiClient.get<NetWorthReport>(`reports/networth?${rangeQuery(range)}`);
  },
);

const initialState: ReportState = {
  range: {
    startMonth: addMonths(getCurrentMonthKey(), -5),
    endMonth: getCurrentMonthKey(),
  },
  preset: '6m',
  spending: null,
  spendingLoading: LoadingState.IDLE,
  incomeExpense: null,
  incomeExpenseLoading: LoadingState.IDLE,
  netWorth: null,
  netWorthLoading: LoadingState.IDLE,
  error: null,
};

const reportSlice = createSlice({
  name: 'reports',
  initialState,
  reducers: {
    // changing the range invalidates all cached reports so the active tab refetches
    setRange: (state, action: PayloadAction<{ range: ReportRange; preset: ReportPreset }>) => {
      state.range = action.payload.range;
      state.preset = action.payload.preset;
      state.spending = null;
      state.spendingLoading = LoadingState.IDLE;
      state.incomeExpense = null;
      state.incomeExpenseLoading = LoadingState.IDLE;
      state.netWorth = null;
      state.netWorthLoading = LoadingState.IDLE;
      state.error = null;
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchSpendingReport.pending, (state) => {
        state.spendingLoading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchSpendingReport.fulfilled, (state, action) => {
        state.spendingLoading = LoadingState.SUCCESS;
        state.spending = action.payload;
      })
      .addCase(fetchSpendingReport.rejected, (state, action) => {
        state.spendingLoading = LoadingState.ERROR;
        state.error = action.error.message ?? null;
      })
      .addCase(fetchIncomeExpenseReport.pending, (state) => {
        state.incomeExpenseLoading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchIncomeExpenseReport.fulfilled, (state, action) => {
        state.incomeExpenseLoading = LoadingState.SUCCESS;
        state.incomeExpense = action.payload;
      })
      .addCase(fetchIncomeExpenseReport.rejected, (state, action) => {
        state.incomeExpenseLoading = LoadingState.ERROR;
        state.error = action.error.message ?? null;
      })
      .addCase(fetchNetWorthReport.pending, (state) => {
        state.netWorthLoading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchNetWorthReport.fulfilled, (state, action) => {
        state.netWorthLoading = LoadingState.SUCCESS;
        state.netWorth = action.payload;
      })
      .addCase(fetchNetWorthReport.rejected, (state, action) => {
        state.netWorthLoading = LoadingState.ERROR;
        state.error = action.error.message ?? null;
      });
  },
});

export const { setRange } = reportSlice.actions;

export default reportSlice.reducer;

// Selectors
export const selectReportRange = (state: RootState) => state.reports.range;
export const selectReportPreset = (state: RootState) => state.reports.preset;
export const selectSpendingReport = (state: RootState) => state.reports.spending;
export const selectSpendingLoading = (state: RootState) => state.reports.spendingLoading;
export const selectIncomeExpenseReport = (state: RootState) => state.reports.incomeExpense;
export const selectIncomeExpenseLoading = (state: RootState) =>
  state.reports.incomeExpenseLoading;
export const selectNetWorthReport = (state: RootState) => state.reports.netWorth;
export const selectNetWorthLoading = (state: RootState) => state.reports.netWorthLoading;
export const selectReportError = (state: RootState) => state.reports.error;
