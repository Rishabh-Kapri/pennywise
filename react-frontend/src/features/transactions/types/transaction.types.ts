import type { LoadingState } from "@/utils";

export const TransactionStatus = {
  MANUAL: 'MANUAL',
  APPROVED: 'APPROVED',
  REJECTED: 'REJECTED',
  UNAPPROVED: 'UNAPPROVED',
} as const;

export type TransactionStatus = typeof TransactionStatus[keyof typeof TransactionStatus];

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
  dedupeHash?: string | null;
  rawBankText?: string | null;
  summary?: string | null;
  transferTransactionId: string | null,
  transferAccountId: string | null,
  tagIds: string[];
  accountName: string;
  accountId: string;
  payeeName: string;
  payeeId: string;
  categoryName: string | null;
  categoryId: string | null;
  deleted?: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface TransactionPrediction {
  id: string;
  budgetId: string;
  transactionId: string;
  emailText?: string | null;
  amount?: number | null;
  account?: string | null;
  accountPrediction?: number | null;
  payee?: string | null;
  payeePrediction?: number | null;
  category?: string | null;
  categoryPrediction?: number | null;
  hasUserCorrected?: boolean | null;
  userCorrectedAccount?: string | null;
  userCorrectedPayee?: string | null;
  userCorrectedCategory?: string | null;
  createdAt?: string;
  updatedAt?: string;
  deleted?: boolean;
}

export interface CipherPrediction {
  id: string;
  budgetId: string;
  transactionId: string;
  emailText?: string | null;
  llmReasoning?: string | null;
  metadata?: unknown;
  amount?: number | null;
  extractedAccount?: string | null;
  extractedPayee?: string | null;
  predictedPayeeId?: string | null;
  predictedCategoryId?: string | null;
  accountConfidence?: number | null;
  payeeConfidence?: number | null;
  categoryConfidence?: number | null;
  source?: string;
  hasUserCorrected?: boolean;
  actualPayeeId?: string | null;
  actualCategoryId?: string | null;
  createdAt?: string;
  updatedAt?: string;
  deleted?: boolean;
}

export interface TransactionPredictionDetails {
  prediction?: TransactionPrediction | null;
  cipherPrediction?: CipherPrediction | null;
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

export interface TransactionStatusDTO {
  id: string;
  status: Extract<TransactionStatus, typeof TransactionStatus.APPROVED | typeof TransactionStatus.REJECTED>;
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

export interface MonthGroupStats {
  count: number;
  totalInflow: number;
  totalOutflow: number;
}

export type ListItem =
  | { type: 'header'; key: string; label: string; stats: MonthGroupStats }
  | { type: 'row'; txn: Transaction; originalIndex: number };
