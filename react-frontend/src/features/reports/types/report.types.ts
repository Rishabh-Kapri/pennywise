import type { LoadingState } from '@/utils';

export interface SpendingCategoryReport {
  categoryId: string;
  name: string;
  total: number;
}

export interface SpendingGroupReport {
  categoryGroupId: string;
  name: string;
  total: number;
  categories: SpendingCategoryReport[];
}

export interface SpendingReport {
  startMonth: string;
  endMonth: string;
  total: number;
  groups: SpendingGroupReport[];
}

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
  liabilities: number;
  netWorth: number;
}

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
  return (
    filters.accountIds.length > 0 || filters.categoryIds.length > 0 || filters.tagIds.length > 0
  );
}

export type ReportPreset = '3m' | '6m' | '12m' | 'ytd' | 'all' | 'custom';

export type ReportTab = 'spending' | 'incomeExpense' | 'networth';

export interface ReportState {
  range: ReportRange;
  preset: ReportPreset;
  filters: ReportFilters;
  spending: SpendingReport | null;
  spendingLoading: LoadingState;
  incomeExpense: IncomeExpenseReport | null;
  incomeExpenseLoading: LoadingState;
  netWorth: NetWorthReport | null;
  netWorthLoading: LoadingState;
  error: string | null;
}
