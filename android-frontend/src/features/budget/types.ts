import type { LoadingState } from '../../utils/constants';

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

export interface BudgetTemplateGroupInput {
  name: string;
  categories: { name: string }[];
}

export interface CreateBudgetPayload {
  name: string;
  templateGroups: BudgetTemplateGroupInput[];
}

export interface BudgetState {
  allBudgets: Budget[];
  selectedBudget: Budget | null;
  selectedMonth: string;
  loading: LoadingState;
  error: string | null;
}
