import { Category } from './category.model';

export interface HttpState<T> {
  isLoading: boolean;
  data?: T;
  error?: any;
}

export interface CategoryGroupData {
  name: string;
  id: string;
  balance: number;
  budgeted: number;
  activity: number;
  categories: Category[];
}

export enum SelectedComponent {
  BUDGET = 'budget',
  ACCOUNTS = 'accounts',
  OTHERS = 'others',
  SETTINGS = 'settings',
}
