import type { LoadingState } from '@/utils';

export interface LoanMetadata {
  accountId: string;
  interestRate: number; // annual percentage (e.g. 6 for 6%)
  originalBalance: number;
  monthlyPayment: number; // minimum required payment
  loanStartDate: string; // ISO date string
  categoryId?: string; // paired budget category for loan payments
}

/** One month of a loan's recorded history, from GET /reports/loans. */
export interface LoanMonth {
  month: string; // YYYY-MM
  payment: number;
  /** The part of the payment that cleared interest. */
  interest: number;
  principal: number;
  /** Principal outstanding after the month's payment. */
  balance: number;
  /** No payment was recorded yet; the scheduled one stood in. */
  assumed: boolean;
}

export interface LoanHistory {
  accountId: string;
  balance: number;
  principalPaid: number;
  interestPaid: number;
  months: LoanMonth[];
}

export interface LoanPayoffProjection {
  month: number;
  date: string;
  principalPayment: number;
  interestPayment: number;
  extraPayment: number;
  remainingBalance: number;
  totalInterestPaid: number;
}

export interface PayoffSimulatorInput {
  currentBalance: number;
  interestRate: number; // annual %
  monthlyPayment: number;
  extraMonthlyPayment?: number;
  oneTimeExtraPayment?: number;
  oneTimeExtraPaymentMonth?: number; // which month to apply the one-time payment
}

export interface PayoffComparison {
  interestSaved: number;
  monthsSaved: number;
  basePayoffMonths: number;
  targetPayoffMonths: number;
  baseTotalInterest: number;
  targetTotalInterest: number;
}

export interface LoanState {
  loanMetadata: Record<string, LoanMetadata>; // keyed by accountId
  loading: LoadingState;
  error: string | null;
}
