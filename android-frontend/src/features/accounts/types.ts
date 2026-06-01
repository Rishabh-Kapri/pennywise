import type { LoadingState } from '../../utils/constants';

export interface Account {
  id?: string;
  budgetId: string;
  name: string;
  type: BudgetAccountType | TrackingAccountType | LoanAccountType;
  closed: boolean;
  balance?: number;
  suffix?: string;
  transferPayeeId?: string;
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}

export const BudgetAccountType = {
  CHECKING: 'checking',
  SAVINGS: 'savings',
  CREDIT_CARD: 'creditCard'
} as const;

export const TrackingAccountType = {
  ASSET: 'asset',
  LIABILITY: 'liability'
} as const;

export const LoanAccountType = {
  LOAN: 'loan',
  MORTGAGE: 'mortgage',
  AUTO_LOAN: 'autoLoan',
  STUDENT_LOAN: 'studentLoan',
  PERSONAL_LOAN: 'personalLoan',
  MEDICAL_DEBT: 'medicalDebt'
} as const;

export type BudgetAccountType = (typeof BudgetAccountType)[keyof typeof BudgetAccountType];
export type TrackingAccountType = (typeof TrackingAccountType)[keyof typeof TrackingAccountType];
export type LoanAccountType = (typeof LoanAccountType)[keyof typeof LoanAccountType];

export interface AccountState {
  selectedAccount: Account | null;
  allAccounts: Account[];
  trackingAccounts: Account[];
  budgetAccounts: Account[];
  loanAccounts: Account[];
  loading: LoadingState;
  error: string | null;
}
