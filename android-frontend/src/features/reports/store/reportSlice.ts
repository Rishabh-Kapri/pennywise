import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { apiClient } from '../../../utils/api';
import { LoadingState } from '../../../utils/constants';
import { getCurrentMonthKey, shiftMonth } from '../../../utils/date';
import { setSelectedBudget } from '../../budget/store/budgetSlice';
import type { CashFlowReport, GroupTreatment, NetWorthReport, ReportPreset, ReportRange, ReportState } from '../types';

export type RangePreset = Exclude<ReportPreset, 'custom'>;

// generous floor for "All"; the backend clamps to actual data
const ALL_TIME_START = '2020-01';

export function presetRange(preset: RangePreset): ReportRange {
  const endMonth = getCurrentMonthKey();
  switch (preset) {
    case '3m':
      return { startMonth: shiftMonth(endMonth, -2), endMonth };
    case '6m':
      return { startMonth: shiftMonth(endMonth, -5), endMonth };
    case '12m':
      return { startMonth: shiftMonth(endMonth, -11), endMonth };
    case 'ytd':
      return { startMonth: `${endMonth.split('-')[0]}-01`, endMonth };
    case 'all':
      return { startMonth: ALL_TIME_START, endMonth };
  }
}

const rangeQuery = (range: ReportRange) =>
  `startMonth=${encodeURIComponent(range.startMonth)}&endMonth=${encodeURIComponent(range.endMonth)}`;

export const fetchCashFlowReport = createAsyncThunk<CashFlowReport, ReportRange>('reports/fetchCashFlow', (range) =>
  apiClient.get<CashFlowReport>(`reports/cashflow?${rangeQuery(range)}`)
);

export const fetchNetWorthReport = createAsyncThunk<NetWorthReport, ReportRange>('reports/fetchNetWorth', (range) =>
  apiClient.get<NetWorthReport>(`reports/networth?${rangeQuery(range)}`)
);

// Group treatments are a per-device preference (the web keeps its own in
// localStorage); group ids are globally unique, so one map serves every budget.
const TREATMENTS_KEY = 'pennywise.reports.groupTreatments';

export const loadGroupTreatments = createAsyncThunk('reports/loadGroupTreatments', async () => {
  try {
    return JSON.parse((await AsyncStorage.getItem(TREATMENTS_KEY)) ?? '{}') as Record<string, GroupTreatment>;
  } catch {
    return {};
  }
});

const initialState: ReportState = {
  range: presetRange('12m'),
  preset: '12m',
  cashFlow: null,
  cashFlowLoading: LoadingState.IDLE,
  groupTreatments: {},
  netWorth: null,
  netWorthLoading: LoadingState.IDLE,
  error: null
};

function invalidate(state: ReportState) {
  state.cashFlow = null;
  state.cashFlowLoading = LoadingState.IDLE;
  state.netWorth = null;
  state.netWorthLoading = LoadingState.IDLE;
  state.error = null;
}

const reportSlice = createSlice({
  name: 'reports',
  initialState,
  reducers: {
    // a new range invalidates every cached report so the visible tab refetches
    setRangePreset: (state, action: PayloadAction<RangePreset>) => {
      state.preset = action.payload;
      state.range = presetRange(action.payload);
      invalidate(state);
    },
    invalidateReports: invalidate,
    setGroupTreatment: (state, action: PayloadAction<{ groupId: string; treatment: GroupTreatment | null }>) => {
      const { groupId, treatment } = action.payload;
      if (treatment === null) delete state.groupTreatments[groupId];
      else state.groupTreatments[groupId] = treatment;
      void AsyncStorage.setItem(TREATMENTS_KEY, JSON.stringify(state.groupTreatments));
    }
  },
  extraReducers: (builder) => {
    builder
      .addCase(setSelectedBudget, invalidate)
      .addCase(loadGroupTreatments.fulfilled, (state, action) => {
        state.groupTreatments = { ...action.payload, ...state.groupTreatments };
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
        state.error = action.error.message ?? 'Failed to load the report';
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
        state.error = action.error.message ?? 'Failed to load net worth';
      });
  }
});

export const { setRangePreset, invalidateReports, setGroupTreatment } = reportSlice.actions;
export default reportSlice.reducer;
