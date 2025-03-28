import { AccountsState } from './states/accounts/accounts.state';
import { BudgetsState } from './states/budget/budget.state';
import { CategoriesState } from './states/categories/categories.state';
import { CategoryGroupsState } from './states/categoryGroups/categoryGroups.state';
import { DictionaryState } from './states/dictionary/dictionary.state';
import { TransactionsState } from './states/transactions/transactions.state';
import { UserState } from './states/user/user.state';

export const DashboardStates = [
  DictionaryState,
  UserState,
  AccountsState,
  BudgetsState,
  CategoriesState,
  CategoryGroupsState,
  TransactionsState,
];
