import { createAsyncThunk, createSelector, createSlice, isAnyOf } from '@reduxjs/toolkit';
import type { RootState } from '@/app/store';
import type { Account } from '@/features/accounts/types/account.types';
import { BudgetAccountType, TrackingAccountType } from '@/features/accounts/types/account.types';
import { setSelectedBudget } from '@/features/budget/store/budgetSlice';
import type { IncomeExpenseReport } from '@/features/reports/types/report.types';
import {
  createTransaction,
  deleteTransactionById,
  fetchAllTransaction,
  updateTransaction,
  updateTransactionStatus,
} from '@/features/transactions/store/transactionSlice';
import { TransactionStatus, type Transaction } from '@/features/transactions/types/transaction.types';
import { apiClient, LoadingState } from '@/utils';
import type { PaginationResponse } from '@/utils/common.constants';
import { addMonths, getTodaysDate } from '@/utils/date.utils';
import type { DashboardState, MonthCashFlow, SpendingGroup, SpendingSummary } from '../types';
import { addDays } from '../utils';

/** Days of history behind the balance chart's longest range. */
export const ACTIVITY_DAYS = 183;
const ACTIVITY_LIMIT = 1000;
/** Months before the selected one that feed the cash-flow averages. */
const AVERAGE_MONTHS = 6;

const initialState: DashboardState = {
  activity: [],
  activityTruncated: false,
  activityStart: '',
  activityLoading: LoadingState.IDLE,
  activityRequestId: null,
  cashFlow: null,
  cashFlowMonth: '',
  cashFlowLoading: LoadingState.IDLE,
  revision: 0,
  error: null,
};

export const fetchDashboardActivity = createAsyncThunk('dashboard/fetchActivity', async () => {
  const startDate = addDays(getTodaysDate(), -ACTIVITY_DAYS);
  const params = new URLSearchParams({ startDate, limit: String(ACTIVITY_LIMIT) });
  const res = await apiClient.get<PaginationResponse<Transaction[]>>(`transactions/normalized?${params}`);
  return {
    startDate,
    transactions: res.data ?? [],
    truncated: Boolean(res.pagination?.nextCursor),
  };
});

export const fetchDashboardCashFlow = createAsyncThunk('dashboard/fetchCashFlow', async (month: string) => {
  const params = new URLSearchParams({ startMonth: addMonths(month, -AVERAGE_MONTHS), endMonth: month });
  return await apiClient.get<IncomeExpenseReport>(`reports/income-expense?${params}`);
});

const transactionsChanged = isAnyOf(
  createTransaction.fulfilled,
  updateTransaction.fulfilled,
  updateTransactionStatus.fulfilled,
  deleteTransactionById.fulfilled,
);

const dashboardSlice = createSlice({
  name: 'dashboard',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchDashboardActivity.pending, (state, action) => {
        state.activityLoading = LoadingState.PENDING;
        state.activityRequestId = action.meta.requestId;
      })
      .addCase(fetchDashboardActivity.fulfilled, (state, action) => {
        if (state.activityRequestId !== action.meta.requestId) return;
        state.activity = action.payload.transactions;
        state.activityTruncated = action.payload.truncated;
        state.activityStart = action.payload.startDate;
        state.activityLoading = LoadingState.SUCCESS;
        state.error = null;
      })
      .addCase(fetchDashboardActivity.rejected, (state, action) => {
        if (state.activityRequestId !== action.meta.requestId) return;
        state.activityLoading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load recent activity';
      })
      .addCase(fetchDashboardCashFlow.pending, (state, action) => {
        state.cashFlowLoading = LoadingState.PENDING;
        state.cashFlowMonth = action.meta.arg;
      })
      .addCase(fetchDashboardCashFlow.fulfilled, (state, action) => {
        // a slower response for a month the user already stepped past
        if (state.cashFlowMonth !== action.meta.arg) return;
        state.cashFlow = action.payload;
        state.cashFlowLoading = LoadingState.SUCCESS;
      })
      .addCase(fetchDashboardCashFlow.rejected, (state, action) => {
        if (state.cashFlowMonth !== action.meta.arg) return;
        state.cashFlowLoading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load cash flow';
      })
      // another budget's numbers must not linger while the new ones load
      .addCase(setSelectedBudget, () => initialState)
      // Websocket pushes and the app-start load refetch the shared list
      // (first page only); edits anywhere change it. Either way, refresh.
      .addCase(fetchAllTransaction.fulfilled, (state, action) => {
        if (!action.meta.arg?.cursor) state.revision += 1;
      })
      .addMatcher(transactionsChanged, (state) => {
        state.revision += 1;
      });
  },
});

export default dashboardSlice.reducer;

// ── Base selectors ──

export const selectDashboardActivity = (state: RootState) => state.dashboard.activity;
export const selectActivityWindow = createSelector(
  [
    (state: RootState) => state.dashboard.activity,
    (state: RootState) => state.dashboard.activityTruncated,
    (state: RootState) => state.dashboard.activityStart,
  ],
  (activity, truncated, start) => {
    // Truncated: only the span the loaded rows cover is complete.
    const reliableStart = truncated && activity.length > 0 ? activity[activity.length - 1].date : start;
    return { start, reliableStart, truncated };
  },
);
export const selectDashboardRevision = (state: RootState) => state.dashboard.revision;
export const selectDashboardError = (state: RootState) => state.dashboard.error;
const selectCashFlowReport = (state: RootState) => state.dashboard.cashFlow;
export const selectCashFlowRefreshing = (state: RootState) =>
  state.dashboard.cashFlowLoading === LoadingState.PENDING && state.dashboard.cashFlow !== null;

/** True once there is something to draw; later refetches keep the old render. */
export const selectDashboardReady = (state: RootState) =>
  state.dashboard.activityLoading === LoadingState.SUCCESS ||
  state.dashboard.activityLoading === LoadingState.ERROR ||
  state.dashboard.cashFlow !== null;

const ACCOUNT_ORDER: Record<string, number> = {
  [BudgetAccountType.CHECKING]: 0,
  [BudgetAccountType.SAVINGS]: 1,
  [TrackingAccountType.ASSET]: 2,
  [BudgetAccountType.CREDIT_CARD]: 3,
};

/** Open budget accounts plus tracked assets: the money that's "yours" day to day. */
export const selectDashboardAccounts = createSelector(
  [(state: RootState) => state.accounts.budgetAccounts, (state: RootState) => state.accounts.trackingAccounts],
  (budgetAccounts, trackingAccounts): Account[] =>
    [...budgetAccounts, ...trackingAccounts.filter((account) => account.type === TrackingAccountType.ASSET)]
      .filter((account) => !account.closed && !account.deleted && account.id)
      .sort((a, b) => (ACCOUNT_ORDER[a.type] ?? 9) - (ACCOUNT_ORDER[b.type] ?? 9) || a.name.localeCompare(b.name)),
);

// ── Cash flow ──

/**
 * The report's own end month drives this, not the selected month, so a
 * month switch keeps showing the last month's figures until the new ones land.
 */
export const selectMonthCashFlow = createSelector(
  [selectCashFlowReport, (state: RootState) => state.budgets.selectedBudget?.metadata?.startingBalPayeeId],
  (report, startingBalPayeeId): MonthCashFlow | null => {
    if (!report) return null;
    const month = report.endMonth;
    // An account's opening balance lands as income; it isn't money earned.
    const opening = report.income.payees.find((payee) => payee.payeeId === startingBalPayeeId)?.amounts ?? {};
    const income = (m: string) => (report.income.totals[m] ?? 0) - (opening[m] ?? 0);
    const spending = (m: string) => report.expense.totals[m] ?? 0;
    const active = (m: string) => income(m) !== 0 || spending(m) !== 0;

    const prevMonth = addMonths(month, -1);
    const earlier = report.months.filter((m) => m < month && active(m));
    const average = (pick: (m: string) => number) =>
      earlier.length ? earlier.reduce((sum, m) => sum + pick(m), 0) / earlier.length : 0;

    return {
      month,
      income: income(month),
      spending: spending(month),
      net: income(month) - spending(month),
      prevIncome: active(prevMonth) ? income(prevMonth) : null,
      prevSpending: active(prevMonth) ? spending(prevMonth) : null,
      avgIncome: average(income),
      avgSpending: average(spending),
      avgMonths: earlier.length,
    };
  },
);

// ── Spending ──

export const selectSpendingSummary = createSelector(
  [
    selectCashFlowReport,
    (state: RootState) => state.categories.allCategoryGroups,
    (state: RootState) => state.budgets.selectedBudget?.metadata?.ccGroupId,
  ],
  (report, categoryGroups, ccGroupId): SpendingSummary | null => {
    if (!report) return null;
    const month = report.endMonth;

    // What was assigned this month, per category and group. Credit-card
    // payments are money moved, not spent, so they stay out like in the report.
    const budgetedByCategory = new Map<string, number>();
    const budgetedByGroup = new Map<string, number>();
    let budgeted = 0;
    for (const group of categoryGroups) {
      if (group.isSystem || group.hidden || group.id === ccGroupId) continue;
      for (const category of group.categories ?? []) {
        if (category.isSystem || category.hidden || !category.id) continue;
        const amount = category.budgeted?.[month] ?? 0;
        budgetedByCategory.set(category.id, amount);
        if (group.id) budgetedByGroup.set(group.id, (budgetedByGroup.get(group.id) ?? 0) + amount);
        budgeted += amount;
      }
    }

    const groups: SpendingGroup[] = report.expense.groups
      .map((group) => ({
        id: group.categoryGroupId,
        name: group.name,
        spent: group.totals[month] ?? 0,
        budgeted: budgetedByGroup.get(group.categoryGroupId) ?? 0,
        categories: group.categories
          .map((category) => ({
            id: category.categoryId,
            name: category.name,
            spent: category.amounts[month] ?? 0,
            budgeted: budgetedByCategory.get(category.categoryId) ?? 0,
          }))
          .filter((category) => category.spent > 0)
          .sort((a, b) => b.spent - a.spent),
      }))
      .filter((group) => group.spent > 0)
      .sort((a, b) => b.spent - a.spent);

    return { month, total: report.expense.totals[month] ?? 0, budgeted, groups };
  },
);

/** Non-transfer transactions in the month with no category yet. */
export const selectUncategorizedCount = createSelector(
  [selectDashboardActivity, (_: RootState, month: string) => month],
  (activity, month) =>
    activity.filter((txn) => txn.date.startsWith(month) && !txn.categoryId && !txn.transferAccountId).length,
);

/** AI-created transactions still waiting for a yes/no. */
export const selectNeedsReviewCount = createSelector(
  [selectDashboardActivity],
  (activity) => activity.filter((txn) => txn.status === TransactionStatus.UNAPPROVED).length,
);
