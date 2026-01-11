import type { LoadingState } from "@/utils";

export const TransactionSource = {
  PENNYWISE: 'PENNYWISE',
  MLP: 'MLP',
} as const;

export type TransactionSource = typeof TransactionSource[keyof typeof TransactionSource];

export interface Transaction {
  id?: string;
  budgetId: string;
  date: string;
  amount?: number;
  outflow: number | null;
  inflow: number | null;
  balance: number;
  note?: string;
  source: TransactionSource;
  transferTransactionId: string | null,
  transferAccountId: string | null,
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
  source: TransactionSource;
}

export interface TransactionState {
  transactions: Transaction[];
  loading: LoadingState;
  error: string | null;
}
