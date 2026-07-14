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

export type ReportPreset = '3m' | '6m' | '12m' | 'ytd' | 'all' | 'custom';

export type ReportTab = 'spending' | 'incomeExpense' | 'networth';

export interface ReportState {
  range: ReportRange;
  preset: ReportPreset;
  spending: SpendingReport | null;
  spendingLoading: LoadingState;
  incomeExpense: IncomeExpenseReport | null;
  incomeExpenseLoading: LoadingState;
  netWorth: NetWorthReport | null;
  netWorthLoading: LoadingState;
  error: string | null;
}
