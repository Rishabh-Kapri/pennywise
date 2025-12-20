import type { LoadingState } from '@/utils';

export interface Budget {
  id?: string;
  name: string;
  createdAt?: string;
  updatedAt?: string;
  isSelected?: boolean;
  metadata?: object;
}

export interface BudgetState {
  allBudgets: Budget[];
  selectedBudget: Budget | null;
  selectedMonth: string;
  loading: LoadingState;
  error: string | null;
}
