import { Account } from 'src/app/models/account.model';
import { Category } from 'src/app/models/category.model';
import { CategoryGroupData } from 'src/app/models/state.model';

interface CategoryChecked extends Category {
  isChecked: boolean;
}
export interface CategoryGroups extends CategoryGroupData {
  isChecked: boolean;
  categories: CategoryChecked[];
}

interface AccountChecked extends Account {
  isChecked: boolean;
}
export interface AccountGroups {
  name: string;
  isChecked: boolean;
  accounts: AccountChecked[];
}
export interface Amount {
  [monthKey: string]: number;
}

export interface IncomeData {
  payee: string;
  amounts: Amount;
}

export interface CategoryData {
  name: string;
  amounts: Amount;
}

export interface CategoryGroupReport {
  groupName: string;
  collapse: boolean;
  categories: CategoryData[];
}

export interface DateRange {
  startDate: string;
  endDate: string;
  monthKey: string;
}
