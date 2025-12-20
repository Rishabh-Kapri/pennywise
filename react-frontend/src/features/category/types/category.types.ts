import type { LoadingState } from "@/utils";

export interface Category {
  id?: string;
  budgetId: string;
  categoryGroupId: string;
  name: string;
  deleted?: boolean;
  createdAt?: string;
  updatedAt?: string;
  hidden?: boolean;
  note?: string | null;
  // goal?: Goal;
  showBudgetInput?: boolean;
  budgeted: Record<string, number>;
  activity?: Record<string, number>;
  balance?: Record<string, number>;
}

export interface CategoryGroup {
  id?: string;
  name: string;
  collapsed: boolean;
  balance: Record<string, number>;
  budgeted: Record<string, number>;
  activity: Record<string, number>;
  categories: Category[];
  isSystem: boolean;
}

export interface CategoryState {
  allCategories: Category[];
  inflowAmount: number;
  loading: LoadingState;
  error: string | null;
}

export interface CategoryGroupState {
  allCategoryGroups: CategoryGroup[];
  collapseAllGroups: boolean;
  inflow: number;
  loading: LoadingState;
  error: string | null;
}

