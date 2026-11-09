import type { LoadingState } from '../../utils/constants';

export interface Payee {
  id?: string;
  budgetId?: string;
  name: string;
  /** id of the account whose transfer payee this is */
  transferAccountId?: string | null;
  defaultCategoryId?: string | null;
  createdAt?: string;
  updatedAt?: string;
  deleted?: boolean;
}

/** Usage of one payee across the budget's transactions (`GET payees/stats`). */
export interface PayeeStats {
  payeeId: string;
  transactionCount: number;
  totalInflow: number;
  /** positive: money that went out */
  totalOutflow: number;
  /** YYYY-MM-DD of the latest transaction */
  lastUsed?: string;
  topCategoryId?: string;
  ruleCount: number;
}

export interface PayeeState {
  allPayees: Payee[];
  loading: LoadingState;
  error: string | null;
  statsById: Record<string, PayeeStats>;
  /** every live rule in the budget, across payees */
  rules: PayeeRule[];
  rulesLoading: LoadingState;
  rulesError: string | null;
}

/**
 * EXACT compares the whole bank string, case-sensitively. PATTERN is a
 * case-insensitive SQL LIKE: `%` is any run of characters, `_` one character.
 */
export type PayeeMatchType = 'EXACT' | 'PATTERN';

export interface PayeeRule {
  id: string;
  budgetId: string;
  payeeId: string;
  payeeName?: string;
  categoryId?: string;
  categoryName?: string;
  matchString: string;
  matchType: PayeeMatchType;
  createdAt?: string;
  updatedAt?: string;
}

export interface PayeeMergeResult {
  targetId: string;
  mergedPayees: number;
  transactions: number;
  rules: number;
  recurring: number;
  embeddings: number;
}
