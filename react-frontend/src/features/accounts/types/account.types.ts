import type { LoadingState } from "@/utils";

export interface Account {
  id?: string;
  budgetId: string;
  name: string;
  type: BudgetAccountType | TrackingAccountType | LoanAccountType;
  closed: boolean;
  balance?: number;
  transferPayeeId?: string; // id of this account's payee
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}

export const BudgetAccountType = {
  CHECKING: 'checking',
  SAVINGS: 'savings',
  CREDIT_CARD: 'creditCard',
} as const;

export const TrackingAccountType = {
  ASSET: 'asset',
  LIABILITY: 'liability',
} as const;

export const LoanAccountType = {
  LOAN: 'loan',
  MORTGAGE: 'mortgage',
  AUTO_LOAN: 'autoLoan',
  STUDENT_LOAN: 'studentLoan',
  PERSONAL_LOAN: 'personalLoan',
  MEDICAL_DEBT: 'medicalDebt',
} as const;

export type BudgetAccountType = typeof BudgetAccountType[keyof typeof BudgetAccountType];
export type TrackingAccountType = typeof TrackingAccountType[keyof typeof TrackingAccountType];
export type LoanAccountType = typeof LoanAccountType[keyof typeof LoanAccountType];

export const BudgetAccountNames: Array<{ value: BudgetAccountType; name: string }> = [
  { value: BudgetAccountType.CHECKING, name: 'Checking' },
  { value: BudgetAccountType.SAVINGS, name: 'Savings' },
  { value: BudgetAccountType.CREDIT_CARD, name: 'Credit Card' },
];

export const TrackingAccountNames: Array<{ value: TrackingAccountType; name: string }> = [
  { value: TrackingAccountType.ASSET, name: 'Asset' },
  { value: TrackingAccountType.LIABILITY, name: 'Liability' },
];

export const LoanAccountNames: Array<{ value: LoanAccountType; name: string }> = [
  { value: LoanAccountType.LOAN, name: 'Loan' },
  { value: LoanAccountType.MORTGAGE, name: 'Mortgage' },
  { value: LoanAccountType.AUTO_LOAN, name: 'Auto Loan' },
  { value: LoanAccountType.STUDENT_LOAN, name: 'Student Loan' },
  { value: LoanAccountType.PERSONAL_LOAN, name: 'Personal Loan' },
  { value: LoanAccountType.MEDICAL_DEBT, name: 'Medical Debt' },
];

export interface AccountState {
  selectedAccount: Account | null;
  allAccounts: Account[];
  trackingAccounts: Account[];
  budgetAccounts: Account[];
  loanAccounts: Account[];
  loading: LoadingState;
  error: string | null;
}

