import type { LoadingState } from "@/utils";

export interface Account {
  id?: string;
  budgetId: string;
  name: string;
  type: BudgetAccountType | TrackingAccountType;
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

export type BudgetAccountType = typeof BudgetAccountType[keyof typeof BudgetAccountType];
export type TrackingAccountType = typeof TrackingAccountType[keyof typeof TrackingAccountType];

export const BudgetAccountNames: Array<{ value: BudgetAccountType; name: string }> = [
  { value: BudgetAccountType.CHECKING, name: 'Checking' },
  { value: BudgetAccountType.SAVINGS, name: 'Savings' },
  { value: BudgetAccountType.CREDIT_CARD, name: 'Credit Card' },
];

export const TrackingAccountNames: Array<{ value: TrackingAccountType; name: string }> = [
  { value: TrackingAccountType.ASSET, name: 'Asset' },
  { value: TrackingAccountType.LIABILITY, name: 'Liability' },
];

export interface AccountState {
  selectedAccount: Account | null;
  allAccounts: Account[];
  trackingAccounts: Account[];
  budgetAccounts: Account[];
  loading: LoadingState;
  error: string | null;
}
