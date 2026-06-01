import type { LoadingState } from '../../utils/constants';

export const TransactionStatus = {
  MANUAL: 'MANUAL',
  APPROVED: 'APPROVED',
  REJECTED: 'REJECTED',
  UNAPPROVED: 'UNAPPROVED'
} as const;

export type TransactionStatus = (typeof TransactionStatus)[keyof typeof TransactionStatus];

export interface Transaction {
  id?: string;
  budgetId: string;
  date: string;
  amount?: number;
  outflow: number | null;
  inflow: number | null;
  balance: number;
  note?: string;
  status?: TransactionStatus;
  transferTransactionId: string | null;
  transferAccountId: string | null;
  tagIds: string[];
  accountName: string;
  accountId: string;
  payeeName: string;
  payeeId: string;
  categoryName: string | null;
  categoryId: string | null;
}

export interface TransactionDTO {
  id?: string;
  budgetId: string;
  accountId: string;
  payeeId: string;
  categoryId: string | null;
  date: string;
  amount: number;
  note?: string;
  status?: TransactionStatus;
  tagIds?: string[];
}

export interface TransactionState {
  transactions: Transaction[];
  optimisticTransactions: Record<string, Transaction>;
  loading: LoadingState;
  loadingMore: LoadingState;
  nextCursor: string | null;
  total: number;
  error: string | null;
}
