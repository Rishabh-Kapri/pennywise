import type { LoadingState } from '../../utils/constants';

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
  allCategoryGroups: CategoryGroup[];
  inflowAmount: number;
  loading: LoadingState;
  inflowLoading: LoadingState;
  error: string | null;
}
