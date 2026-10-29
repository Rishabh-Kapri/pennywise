import type { LoadingState } from '../../utils/constants';

export interface LoanMetadata {
  accountId: string;
  interestRate: number;
  originalBalance: number;
  monthlyPayment: number;
  loanStartDate: string;
  categoryId?: string;
}

export interface LoanState {
  loanMetadata: Record<string, LoanMetadata>;
  loading: LoadingState;
  error: string | null;
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

/**
 * A loan amortized server-side from `loan_metadata` and the payments booked
 * to the loan account: each payment clears accrued interest first, only the
 * rest reduces principal.
 */
export interface LoanHistory {
  accountId: string;
  balance: number;
  principalPaid: number;
  interestPaid: number;
  months: LoanMonth[];
}

export interface PayoffInput {
  currentBalance: number;
  /** annual % */
  interestRate: number;
  monthlyPayment: number;
  extraMonthlyPayment?: number;
  oneTimeExtraPayment?: number;
}
