import { createAsyncThunk, createSelector, createSlice, isAnyOf } from '@reduxjs/toolkit';
import type { RootState } from '../../../app/store';
import { apiClient } from '../../../utils/api';
import { LoadingState, type PaginationResponse } from '../../../utils/constants';
import { addDays, shiftMonth, todayKey } from '../../../utils/date';
import { BudgetAccountType, TrackingAccountType, type Account } from '../../accounts/types';
import { setSelectedBudget } from '../../budget/store/budgetSlice';
import type { IncomeExpenseReport } from '../../reports/types';
import {
  createTransaction,
  deleteTransactionById,
  fetchAllTransactions,
  updateTransaction,
  updateTransactionStatus
} from '../../transactions/store/transactionSlice';
import { TransactionStatus, type Transaction } from '../../transactions/types';

// Port of react-frontend `features/dashboard/store`. The dashboard keeps its
// own transaction window: other screens reload the shared transactions slice
// with a 30-row or filtered page.

/** Days of history behind the balance chart's longest range. */
export const ACTIVITY_DAYS = 183;
const ACTIVITY_LIMIT = 1000;
/** Months before the selected one that feed the cash-flow averages. */
const AVERAGE_MONTHS = 6;

interface DashboardState {
  activity: Transaction[];
  activityTruncated: boolean;
  activityStart: string;
  activityLoading: LoadingState;
  activityRequestId: string | null;
  cashFlow: IncomeExpenseReport | null;
  cashFlowMonth: string;
  cashFlowLoading: LoadingState;
  /** bumped whenever transactions change elsewhere, so the dashboard refetches */
  revision: number;
  error: string | null;
}

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
  error: null
};

export const fetchDashboardActivity = createAsyncThunk('dashboard/fetchActivity', async () => {
  const startDate = addDays(todayKey(), -ACTIVITY_DAYS);
  const res = await apiClient.get<PaginationResponse<Transaction[]>>(
    `transactions/normalized?startDate=${startDate}&limit=${ACTIVITY_LIMIT}`
  );
  return { startDate, transactions: res.data ?? [], truncated: Boolean(res.pagination?.nextCursor) };
});

export const fetchDashboardCashFlow = createAsyncThunk('dashboard/fetchCashFlow', async (month: string) =>
  apiClient.get<IncomeExpenseReport>(
    `reports/income-expense?startMonth=${shiftMonth(month, -AVERAGE_MONTHS)}&endMonth=${month}`
  )
);

const transactionsChanged = isAnyOf(
  createTransaction.fulfilled,
  updateTransaction.fulfilled,
  updateTransactionStatus.fulfilled,
  deleteTransactionById.fulfilled
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
      // websocket pushes and the app-start load refetch the shared list's
      // first page; edits anywhere change it. Either way, refresh.
      .addCase(fetchAllTransactions.fulfilled, (state, action) => {
        if (!action.meta.arg?.cursor) state.revision += 1;
      })
      .addMatcher(transactionsChanged, (state) => {
        state.revision += 1;
      });
  }
});

export default dashboardSlice.reducer;

const ACCOUNT_ORDER: Record<string, number> = {
  [BudgetAccountType.CHECKING]: 0,
  [BudgetAccountType.SAVINGS]: 1,
  [TrackingAccountType.ASSET]: 2,
  [BudgetAccountType.CREDIT_CARD]: 3
};

/** Open budget accounts plus tracked assets: the money that's "yours" day to day. */
export const selectDashboardAccounts = createSelector(
  [(state: RootState) => state.accounts.budgetAccounts, (state: RootState) => state.accounts.trackingAccounts],
  (budgetAccounts, trackingAccounts): Account[] =>
    [...budgetAccounts, ...trackingAccounts.filter((account) => account.type === TrackingAccountType.ASSET)]
      .filter((account) => !account.closed && !account.deleted && account.id)
      .sort((a, b) => (ACCOUNT_ORDER[a.type] ?? 9) - (ACCOUNT_ORDER[b.type] ?? 9) || a.name.localeCompare(b.name))
);

export const selectActivityWindow = createSelector(
  [
    (state: RootState) => state.dashboard.activity,
    (state: RootState) => state.dashboard.activityTruncated,
    (state: RootState) => state.dashboard.activityStart
  ],
  (activity, truncated, start) => ({
    start,
    // truncated: only the span the loaded rows cover is complete
    reliableStart: truncated && activity.length > 0 ? activity[activity.length - 1].date : start
  })
);

export interface MonthCashFlow {
  month: string;
  income: number;
  spending: number;
  net: number;
  prevIncome: number | null;
  prevSpending: number | null;
  avgIncome: number;
  avgSpending: number;
  avgMonths: number;
}

/** The report's end month drives this, so a month switch keeps the old figures until the new ones land. */
export const selectMonthCashFlow = createSelector(
  [(state: RootState) => state.dashboard.cashFlow, (state: RootState) => state.budgets.selectedBudget?.metadata?.startingBalPayeeId],
  (report, startingBalPayeeId): MonthCashFlow | null => {
    if (!report) return null;
    const month = report.endMonth;
    // an account's opening balance lands as income; it isn't money earned
    const opening = report.income.payees.find((payee) => payee.payeeId === startingBalPayeeId)?.amounts ?? {};
    const income = (m: string) => (report.income.totals[m] ?? 0) - (opening[m] ?? 0);
    const spending = (m: string) => report.expense.totals[m] ?? 0;
    const active = (m: string) => income(m) !== 0 || spending(m) !== 0;
    const prevMonth = shiftMonth(month, -1);
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
      avgMonths: earlier.length
    };
  }
);

export interface SpendingCategory {
  id: string;
  name: string;
  spent: number;
  budgeted: number;
}

export interface SpendingGroup extends SpendingCategory {
  categories: SpendingCategory[];
}

export const selectSpendingSummary = createSelector(
  [
    (state: RootState) => state.dashboard.cashFlow,
    (state: RootState) => state.categories.allCategoryGroups,
    (state: RootState) => state.budgets.selectedBudget?.metadata?.ccGroupId
  ],
  (report, categoryGroups, ccGroupId) => {
    if (!report) return null;
    const month = report.endMonth;
    // credit-card payments are money moved, not spent
    const budgetedByCategory = new Map<string, number>();
    const budgetedByGroup = new Map<string, number>();
    let budgeted = 0;
    for (const group of categoryGroups) {
      if (group.isSystem || group.id === ccGroupId) continue;
      for (const category of group.categories ?? []) {
        if (category.hidden || !category.id) continue;
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
            budgeted: budgetedByCategory.get(category.categoryId) ?? 0
          }))
          .filter((category) => category.spent > 0)
          .sort((a, b) => b.spent - a.spent)
      }))
      .filter((group) => group.spent > 0)
      .sort((a, b) => b.spent - a.spent);
    return { month, total: report.expense.totals[month] ?? 0, budgeted, groups };
  }
);

/** End-of-day balance of the given accounts, walked back from their current balance. */
export function buildBalanceHistory(
  currentBalance: number,
  transactions: Transaction[],
  accountIds: Set<string>,
  start: string,
  today: string
): { date: string; balance: number; change: number }[] {
  const netByDate = new Map<string, number>();
  for (const txn of transactions) {
    if (!accountIds.has(txn.accountId)) continue;
    netByDate.set(txn.date, (netByDate.get(txn.date) ?? 0) + (txn.inflow ?? 0) - (txn.outflow ?? 0));
  }
  // account balances already include anything dated after today
  let balance = currentBalance;
  for (const [date, net] of netByDate) if (date > today) balance -= net;

  const points: { date: string; balance: number; change: number }[] = [];
  for (let date = today; date >= start; date = addDays(date, -1)) {
    const change = netByDate.get(date) ?? 0;
    points.push({ date, balance, change });
    balance -= change;
  }
  points.reverse();
  // before the accounts' first transaction the walk sits at zero
  const firstActive = points.findIndex((point) => point.change !== 0);
  if (firstActive > 0 && points.slice(0, firstActive).every((point) => Math.abs(point.balance) < 0.5)) {
    return points.slice(firstActive);
  }
  return points;
}

export const selectNeedsReviewCount = createSelector(
  [(state: RootState) => state.dashboard.activity],
  (activity) => activity.filter((txn) => txn.status === TransactionStatus.UNAPPROVED).length
);
