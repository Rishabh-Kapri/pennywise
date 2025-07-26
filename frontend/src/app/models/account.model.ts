export interface Account {
  id?: string;
  budgetId: string;
  name: string;
  type: BudgetAccountType | TrackingAccountType;
  closed: boolean;
  balance: number;
  transferPayeeId?: string; // id of this account's payee
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}

export enum BudgetAccountType {
  CHECKING = 'checking',
  SAVINGS = 'savings',
  CREDIT_CARD = 'creditCard',
}

export enum TrackingAccountType {
  ASSET = 'asset',
  LIABILITY = 'liability',
}

export const BudgetAccountNames: Array<{ value: BudgetAccountType; name: string }> = [
  { value: BudgetAccountType.CHECKING, name: 'Checking' },
  { value: BudgetAccountType.SAVINGS, name: 'Savings' },
  { value: BudgetAccountType.CREDIT_CARD, name: 'Credit Card' },
];
export const TrackingAccountNames: Array<{ value: TrackingAccountType; name: string }> = [
  { value: TrackingAccountType.ASSET, name: 'Asset' },
  { value: TrackingAccountType.LIABILITY, name: 'Liability' },
];
