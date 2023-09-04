export interface Account {
  id?: string;
  budgetId: string;
  name: string;
  type: AccountType;
  closed: boolean;
  balance: number;
  transferPayeeId?: string; // use from the payee collection
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}

export enum AccountType {
  CHECKING = 'checking',
  SAVINGS = 'savings',
  CREDIT_CARD = 'creditCard',
  ASSET = 'asset',
  LIABILITY = 'liability',
}

export const AccountTypeNames: Array<{ value: AccountType; name: string }> = [
  { value: AccountType.CHECKING, name: 'Checking' },
  { value: AccountType.SAVINGS, name: 'Savings' },
  { value: AccountType.CREDIT_CARD, name: 'Credit Card' },
  { value: AccountType.ASSET, name: 'Asset' },
  { value: AccountType.LIABILITY, name: 'Liability' },
];
