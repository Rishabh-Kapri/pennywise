import type { LoadingState } from '@/utils';

export interface LoanMetadata {
  accountId: string;
  interestRate: number; // annual percentage (e.g. 6 for 6%)
  originalBalance: number;
  monthlyPayment: number; // minimum required payment
  loanStartDate: string; // ISO date string
  categoryId?: string; // paired budget category for loan payments
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
