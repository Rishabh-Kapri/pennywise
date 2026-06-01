import type { LoadingState } from '../../utils/constants';

export interface Payee {
  id?: string;
  name: string;
  budgetId?: string;
  transferAccountId?: string | null;
  defaultCategoryId?: string | null;
  deleted?: boolean;
}

export interface PayeeRule {
  id?: string;
  payeeId: string;
  categoryId?: string | null;
  matchString: string;
  matchType: 'EXACT' | 'PATTERN';
}

export interface PayeeState {
  allPayees: Payee[];
  loading: LoadingState;
  error: string | null;
}
