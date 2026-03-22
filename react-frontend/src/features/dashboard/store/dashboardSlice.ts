import { createSelector, createSlice } from '@reduxjs/toolkit';
import type { RootState } from '@/app/store';
import type {
  DashboardState,
  DashboardStats,
  SpendingTrend,
  BudgetHealthSummary,
  CategoryHealth,
} from '../types';
import { LoadingState } from '@/utils';

const initialState: DashboardState = {
  stats: {
    totalIncome: 0,
    totalExpenses: 0,
    netWorth: 0,
  },
  spendingTrends: [],
  budgetHealth: {
    healthyCount: 0,
    warningCount: 0,
    dangerCount: 0,
    categories: [],
  },
  loading: LoadingState.IDLE,
  error: null,
};

const dashboardSlice = createSlice({
  name: 'dashboard',
  initialState,
  reducers: {},
});

export default dashboardSlice.reducer;

// Base selectors
const selectTransactions = (state: RootState) => state.transactions.transactions;
const selectAccounts = (state: RootState) => state.accounts.allAccounts;
const selectCategoryGroups = (state: RootState) =>
  state.categories.allCategoryGroups;
const selectSelectedMonth = (state: RootState) => state.budgets.selectedMonth;

/**
 * Compute dashboard stats from transactions and accounts
 */
export const selectDashboardStats = createSelector(
  [selectTransactions, selectAccounts],
  (transactions, accounts): DashboardStats => {
    const totalIncome = transactions.reduce(
      (sum, txn) => sum + (txn.inflow ?? 0),
      0
    );
    const totalExpenses = transactions.reduce(
      (sum, txn) => sum + (txn.outflow ?? 0),
      0
    );
    const netWorth = accounts.reduce(
      (sum, acc) => sum + (acc.balance ?? 0),
      0
    );

    return {
      totalIncome,
      totalExpenses,
      netWorth,
    };
  }
);

/**
 * Compute spending trends (last 6 months) from transactions
 */
export const selectSpendingTrends = createSelector(
  [selectTransactions],
  (transactions): SpendingTrend[] => {
    const monthLabels = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    const monthMap = new Map<string, { income: number; expenses: number }>();

    // Get last 6 months
    const now = new Date();
    const months: string[] = [];
    for (let i = 5; i >= 0; i--) {
      const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
      const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`;
      months.push(key);
      monthMap.set(key, { income: 0, expenses: 0 });
    }

    // Aggregate transactions by month
    transactions.forEach((txn) => {
      const txnMonth = txn.date.substring(0, 7); // "YYYY-MM"
      if (monthMap.has(txnMonth)) {
        const data = monthMap.get(txnMonth)!;
        data.income += txn.inflow ?? 0;
        data.expenses += txn.outflow ?? 0;
      }
    });

    return months.map((month) => {
      const data = monthMap.get(month)!;
      const monthIndex = parseInt(month.split('-')[1], 10) - 1;
      return {
        month,
        monthLabel: monthLabels[monthIndex],
        income: data.income,
        expenses: data.expenses,
      };
    });
  }
);

/**
 * Compute budget health from category groups for the selected month
 */
export const selectBudgetHealth = createSelector(
  [selectCategoryGroups, selectSelectedMonth],
  (categoryGroups, selectedMonth): BudgetHealthSummary => {
    const categories: CategoryHealth[] = [];

    categoryGroups.forEach((group) => {
      if (group.isSystem) return; // Skip system categories

      group.categories.forEach((cat) => {
        const budgeted = cat.budgeted?.[selectedMonth] ?? 0;
        const activity = cat.activity?.[selectedMonth] ?? 0;
        const spent = Math.abs(activity);
        const remaining = budgeted - spent;
        const percentUsed = budgeted > 0 ? (spent / budgeted) * 100 : 0;

        let status: CategoryHealth['status'] = 'healthy';
        if (remaining < 0) {
          status = 'danger';
        } else if (percentUsed >= 80) {
          status = 'warning';
        }

        // Only include categories with budgets or spending
        if (budgeted > 0 || spent > 0) {
          categories.push({
            id: cat.id ?? '',
            name: cat.name,
            budgeted,
            spent,
            remaining,
            percentUsed: Math.min(percentUsed, 100),
            status,
          });
        }
      });
    });

    const healthyCount = categories.filter((c) => c.status === 'healthy').length;
    const warningCount = categories.filter((c) => c.status === 'warning').length;
    const dangerCount = categories.filter((c) => c.status === 'danger').length;

    return {
      healthyCount,
      warningCount,
      dangerCount,
      categories,
    };
  }
);

/**
 * Select recent transactions (last 10)
 */
export const selectRecentTransactions = createSelector(
  [selectTransactions],
  (transactions) => {
    return [...transactions]
      .sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime())
      .slice(0, 10);
  }
);

// Loading state selectors
export const selectDashboardLoading = createSelector(
  [
    (state: RootState) => state.accounts.loading,
    (state: RootState) => state.transactions.loading,
    (state: RootState) => state.categories.loading,
  ],
  (accountsLoading, txnLoading, catLoading): LoadingState => {
    if (
      accountsLoading === LoadingState.PENDING ||
      txnLoading === LoadingState.PENDING ||
      catLoading === LoadingState.PENDING
    ) {
      return LoadingState.PENDING;
    }
    if (
      accountsLoading === LoadingState.ERROR ||
      txnLoading === LoadingState.ERROR ||
      catLoading === LoadingState.ERROR
    ) {
      return LoadingState.ERROR;
    }
    if (
      accountsLoading === LoadingState.SUCCESS &&
      txnLoading === LoadingState.SUCCESS &&
      catLoading === LoadingState.SUCCESS
    ) {
      return LoadingState.SUCCESS;
    }
    return LoadingState.IDLE;
  }
);
