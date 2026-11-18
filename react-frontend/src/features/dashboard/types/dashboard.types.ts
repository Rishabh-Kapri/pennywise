import type { LoadingState } from '@/utils';
import type { Transaction } from '@/features/transactions/types/transaction.types';
import type { IncomeExpenseReport } from '@/features/reports/types/report.types';

/** End-of-day balance for the balance chart. */
export interface BalancePoint {
  date: string; // YYYY-MM-DD
  balance: number;
  /** Net movement on this day. */
  change: number;
}

/** The selected month's money in and out, with context. */
export interface MonthCashFlow {
  month: string;
  income: number;
  spending: number;
  net: number;
  /** Null when the previous month had no activity. */
  prevIncome: number | null;
  prevSpending: number | null;
  avgIncome: number;
  avgSpending: number;
  /** Months that went into the averages (ones with any activity). */
  avgMonths: number;
}

export interface SpendingCategory {
  id: string;
  name: string;
  spent: number;
  /** Assigned this month. */
  budgeted: number;
}

export interface SpendingGroup {
  id: string;
  name: string;
  spent: number;
  budgeted: number;
  categories: SpendingCategory[];
}

export interface SpendingSummary {
  month: string;
  total: number;
  /** Assigned this month across spending categories. */
  budgeted: number;
  groups: SpendingGroup[];
}

export interface DashboardState {
  /**
   * The last ~6 months of transactions, newest first. The dashboard keeps its
   * own copy: other screens reload the shared transactions slice with 30 rows.
   */
  activity: Transaction[];
  /** The window held more transactions than were loaded. */
  activityTruncated: boolean;
  /** The first date the activity window covers. */
  activityStart: string;
  activityLoading: LoadingState;
  activityRequestId: string | null;
  /** Income vs expense for the selected month and the months before it. */
  cashFlow: IncomeExpenseReport | null;
  cashFlowMonth: string;
  cashFlowLoading: LoadingState;
  /** Bumped whenever transactions change elsewhere, so the dashboard refetches. */
  revision: number;
  error: string | null;
}
