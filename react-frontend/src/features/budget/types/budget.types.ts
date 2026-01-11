import type { LoadingState } from '@/utils';

export interface Budget {
  id?: string;
  name: string;
  createdAt?: string;
  updatedAt?: string;
  isSelected?: boolean;
  metadata?: {
    inflowCategoryId: string;
    startingBalPayeeId: string;
    ccGroupId: string;
  };
}

export interface BudgetState {
  allBudgets: Budget[];
  selectedBudget: Budget | null;
  selectedMonth: string;
  loading: LoadingState;
  error: string | null;
}
