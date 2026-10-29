import type { LoadingState } from '@/utils';

export interface PayeeMonthlyAmounts {
  payeeId: string;
  name: string;
  amounts: Record<string, number>;
}

export interface IncomeMatrix {
  payees: PayeeMonthlyAmounts[];
  totals: Record<string, number>;
}

export interface CategoryMonthlyAmounts {
  categoryId: string;
  name: string;
  amounts: Record<string, number>;
}

export interface ExpenseGroupMonthly {
  categoryGroupId: string;
  name: string;
  totals: Record<string, number>;
  categories: CategoryMonthlyAmounts[];
}

export interface ExpenseMatrix {
  groups: ExpenseGroupMonthly[];
  totals: Record<string, number>;
}

export interface IncomeExpenseReport {
  startMonth: string;
  endMonth: string;
  months: string[];
  income: IncomeMatrix;
  expense: ExpenseMatrix;
  net: Record<string, number>;
}

export interface NetWorthPoint {
  month: string;
  assets: number;
  /** The tracking "asset" accounts within assets. */
  investments: number;
  liabilities: number;
  /** Amortized loan balances within liabilities (negative). */
  loans: number;
  netWorth: number;
}

/** Money moved between on-budget accounts and one off-budget account. */
export interface AccountFlow {
  accountId: string;
  name: string;
  type: string;
  kind: 'asset' | 'debt';
  closed: boolean;
  /** Came into on-budget accounts, by month. */
  in: Record<string, number>;
  /** Left on-budget accounts (positive), by month. */
  out: Record<string, number>;
  /** Inflows filed under a regular category instead of Ready to Assign. */
  recategorizedCount: number;
  recategorizedAmount: number;
}

/**
 * GET /reports/cashflow. Income is earned income only (no transfers, no
 * opening balances); spending excludes transfers; every transfer with an
 * off-budget account is in `transfers`, whatever category it was filed under.
 */
export interface CashFlowReport {
  startMonth: string;
  endMonth: string;
  months: string[];
  income: IncomeMatrix;
  spending: ExpenseMatrix;
  transfers: AccountFlow[];
  openingBalances: Record<string, number>;
}

/**
 * How a category group counts in the reports: ordinary spending, spending
 * that won't recur (shown apart in the spend rate), or money set aside
 * (counted with investments, not spending).
 */
export type GroupTreatment = 'spending' | 'oneOff' | 'saving';

export interface NetWorthReport {
  startMonth: string;
  endMonth: string;
  months: NetWorthPoint[];
}

export interface ReportRange {
  startMonth: string;
  endMonth: string;
}

export interface ReportFilters {
  accountIds: string[];
  accountNames: string[];
  categoryIds: string[];
  categoryNames: string[];
  tagIds: string[];
  tagNames: string[];
}

export const EMPTY_REPORT_FILTERS: ReportFilters = {
  accountIds: [],
  accountNames: [],
  categoryIds: [],
  categoryNames: [],
  tagIds: [],
  tagNames: [],
};

export function hasActiveReportFilters(filters: ReportFilters): boolean {
  return filters.accountIds.length > 0 || filters.categoryIds.length > 0 || filters.tagIds.length > 0;
}

export type ReportPreset = '3m' | '6m' | '12m' | 'ytd' | 'all' | 'custom';

export type ReportTab = 'overview' | 'spending' | 'income' | 'investing' | 'networth';

export interface ReportState {
  range: ReportRange;
  preset: ReportPreset;
  filters: ReportFilters;
  cashFlow: CashFlowReport | null;
  cashFlowLoading: LoadingState;
  /** Overrides of the name-based default, keyed by category group id. */
  groupTreatments: Record<string, GroupTreatment>;
  netWorth: NetWorthReport | null;
  netWorthLoading: LoadingState;
  error: string | null;
}
