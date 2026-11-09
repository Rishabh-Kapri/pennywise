import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import {
  EMPTY_REPORT_FILTERS,
  type CashFlowReport,
  type GroupTreatment,
  type NetWorthReport,
  type ReportFilters,
  type ReportPreset,
  type ReportRange,
  type ReportState,
} from '../types/report.types';
import { apiClient, LoadingState } from '@/utils';
import { addMonths, getCurrentMonthKey } from '@/utils/date.utils';
import { setSelectedBudget } from '@/features/budget/store/budgetSlice';
import type { RootState } from '@/app';

export interface ReportQueryArgs {
  range: ReportRange;
  filters: ReportFilters;
}

function reportQuery({ range, filters }: ReportQueryArgs): string {
  const params = new URLSearchParams();
  params.set('startMonth', range.startMonth);
  params.set('endMonth', range.endMonth);
  if (filters.accountIds.length > 0) {
    params.set('accountIds', filters.accountIds.join(','));
  }
  if (filters.categoryIds.length > 0) {
    params.set('categoryIds', filters.categoryIds.join(','));
  }
  if (filters.tagIds.length > 0) {
    params.set('tagIds', filters.tagIds.join(','));
  }
  return params.toString();
}

export const fetchCashFlowReport = createAsyncThunk<CashFlowReport, ReportQueryArgs>(
  'reports/fetchCashFlow',
  async (args) => {
    return await apiClient.get<CashFlowReport>(`reports/cashflow?${reportQuery(args)}`);
  },
);

export const fetchNetWorthReport = createAsyncThunk<NetWorthReport, ReportQueryArgs>(
  'reports/fetchNetWorth',
  async (args) => {
    return await apiClient.get<NetWorthReport>(`reports/networth?${reportQuery(args)}`);
  },
);

// Group treatments are a per-device preference; group ids are globally
// unique, so one map serves every budget.
const TREATMENTS_KEY = 'pennywise.reports.groupTreatments';

function loadTreatments(): Record<string, GroupTreatment> {
  try {
    return JSON.parse(localStorage.getItem(TREATMENTS_KEY) ?? '{}') as Record<string, GroupTreatment>;
  } catch {
    return {};
  }
}

const initialState: ReportState = {
  range: {
    startMonth: addMonths(getCurrentMonthKey(), -11),
    endMonth: getCurrentMonthKey(),
  },
  preset: '12m',
  filters: EMPTY_REPORT_FILTERS,
  cashFlow: null,
  cashFlowLoading: LoadingState.IDLE,
  groupTreatments: loadTreatments(),
  netWorth: null,
  netWorthLoading: LoadingState.IDLE,
  error: null,
};

const reportSlice = createSlice({
  name: 'reports',
  initialState,
  reducers: {
    // changing the range or filters invalidates all cached reports so the
    // active tab refetches
    setRange: (state, action: PayloadAction<{ range: ReportRange; preset: ReportPreset }>) => {
      state.range = action.payload.range;
      state.preset = action.payload.preset;
      invalidateReports(state);
    },
    setReportFilters: (state, action: PayloadAction<ReportFilters>) => {
      state.filters = action.payload;
      invalidateReports(state);
    },
    setGroupTreatment: (state, action: PayloadAction<{ groupId: string; treatment: GroupTreatment | null }>) => {
      const { groupId, treatment } = action.payload;
      if (treatment === null) {
        delete state.groupTreatments[groupId];
      } else {
        state.groupTreatments[groupId] = treatment;
      }
      localStorage.setItem(TREATMENTS_KEY, JSON.stringify(state.groupTreatments));
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(setSelectedBudget, (state) => {
        // filters hold the old budget's account/category ids
        state.filters = EMPTY_REPORT_FILTERS;
        invalidateReports(state);
      })
      .addCase(fetchCashFlowReport.pending, (state) => {
        state.cashFlowLoading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchCashFlowReport.fulfilled, (state, action) => {
        state.cashFlowLoading = LoadingState.SUCCESS;
        state.cashFlow = action.payload;
      })
      .addCase(fetchCashFlowReport.rejected, (state, action) => {
        state.cashFlowLoading = LoadingState.ERROR;
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

function invalidateReports(state: ReportState) {
  state.cashFlow = null;
  state.cashFlowLoading = LoadingState.IDLE;
  state.netWorth = null;
  state.netWorthLoading = LoadingState.IDLE;
  state.error = null;
}

export const { setRange, setReportFilters, setGroupTreatment } = reportSlice.actions;

export default reportSlice.reducer;

// Selectors
export const selectReportRange = (state: RootState) => state.reports.range;
export const selectReportFilters = (state: RootState) => state.reports.filters;
export const selectReportPreset = (state: RootState) => state.reports.preset;
export const selectCashFlowReport = (state: RootState) => state.reports.cashFlow;
export const selectCashFlowLoading = (state: RootState) => state.reports.cashFlowLoading;
export const selectGroupTreatments = (state: RootState) => state.reports.groupTreatments;
export const selectNetWorthReport = (state: RootState) => state.reports.netWorth;
export const selectNetWorthLoading = (state: RootState) => state.reports.netWorthLoading;
export const selectReportError = (state: RootState) => state.reports.error;
