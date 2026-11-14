import type { LoadingState } from '@/utils';
import type { BudgetOnboarding } from '@/features/onboarding/types';
import type { BudgetRole } from '@/features/sharing/types';

export interface Budget {
  id?: string;
  name: string;
  createdAt?: string;
  updatedAt?: string;
  isSelected?: boolean;
  /** The signed-in user's role in this budget. */
  role?: BudgetRole;
  /** How many people are in the budget, the user included. */
  memberCount?: number;
  metadata?: {
    inflowCategoryId: string;
    startingBalPayeeId: string;
    ccGroupId: string;
  };
  onboarding?: BudgetOnboarding;
}

export interface BudgetState {
  allBudgets: Budget[];
  selectedBudget: Budget | null;
  selectedMonth: string;
  loading: LoadingState;
  error: string | null;
}
