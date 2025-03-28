import { Injectable } from '@angular/core';
import { Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { Action, NgxsOnInit, Selector, State, StateContext, Store } from '@ngxs/store';
import { CategoryGroup } from 'src/app/models/catergoryGroup';
import { CategoryGroupData } from 'src/app/models/state.model';
import { CategoryGroupsFirestore } from 'src/app/services/databases/categoryGroups.firestore';
import { CategoryGroupsActions } from './categoryGroups.action';
import { CategoriesState } from '../categories/categories.state';
import { TransactionsState } from '../transactions/transactions.state';
import { MASTER_CATEGORY_GROUP_NAME } from 'src/app/constants/general';
import { BudgetsState } from '../budget/budget.state';
import { HelperService } from 'src/app/services/helper.service';
import { Transaction } from 'src/app/models/transaction.model';
import { AccountsState } from '../accounts/accounts.state';
import { Category } from 'src/app/models/category.model';

export interface CategoryGroupsStateModel {
  allCategoryGroups: CategoryGroup[];
  categoryGroups: CategoryGroupData[];
  collapseAllGroups: boolean;
}
@State<CategoryGroupsStateModel>({
  name: 'categoryGroups',
  defaults: {
    allCategoryGroups: [],
    categoryGroups: [],
    collapseAllGroups: false,
  },
})
@Injectable()
export class CategoryGroupsState implements NgxsOnInit {
  @Selector()
  static getAllCategories(state: CategoryGroupsStateModel): CategoryGroup[] {
    return state.allCategoryGroups;
  }
  @Selector()
  static getCategoryGroups(state: CategoryGroupsStateModel): CategoryGroupData[] {
    return state.categoryGroups;
  }

  constructor(
    private ngxsStore: Store,
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private categoryGroupsFs: CategoryGroupsFirestore,
    private helperService: HelperService,
  ) {}

  ngxsOnInit(ctx: StateContext<any>): void {
    this.ngxsFirestoreConnect.connect(CategoryGroupsActions.GetAllCategoryGroups, {
      to: () => this.categoryGroupsFs.collection$(),
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(CategoryGroupsActions.GetAllCategoryGroups))
  getAllBudgets(
    ctx: StateContext<CategoryGroupsStateModel>,
    { action, payload }: Emitted<CategoryGroupsActions.GetAllCategoryGroups, CategoryGroup[]>,
  ) {
    ctx.setState({
      ...ctx.getState(),
      allCategoryGroups: payload,
    });
  }

  @Action(CategoryGroupsActions.SetCategoryGroupData)
  setCategoryGroupData(ctx: StateContext<CategoryGroupsStateModel>) {
    console.log('setCategoryGroupData', ctx);
    const categories = <Category[]>(
      JSON.parse(JSON.stringify(this.ngxsStore.selectSnapshot(CategoriesState.getAllCategories)))
    );
    const transactions = this.ngxsStore.selectSnapshot(TransactionsState.getAllTransactions);
    const selectedMonth = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth);
    const ccAccounts = this.ngxsStore.selectSnapshot(AccountsState.getCreditCardAccounts);
    const state = ctx.getState();
    console.log(state);

    const categoryGroupData: CategoryGroupData[] = [];

    for (const group of state.allCategoryGroups) {
      if (group.name !== MASTER_CATEGORY_GROUP_NAME) {
        const groupCategories = categories.filter((cat) => cat.categoryGroupId === group.id && !cat.hidden);
        const data: CategoryGroupData = {
          name: group.name,
          id: group.id!,
          collapsed: state.collapseAllGroups,
          balance: {
            [selectedMonth]: this.helperService.reduceCategoriesAmount(groupCategories, 'balance', selectedMonth),
          },
          activity: {
            [selectedMonth]: this.helperService.reduceCategoriesAmount(groupCategories, 'activity', selectedMonth),
          },
          budgeted: {
            [selectedMonth]: this.helperService.reduceCategoriesAmount(groupCategories, 'budgeted', selectedMonth),
          },
          categories: [
            ...groupCategories.map((category) => {
              const currentMonthTxns = this.helperService.filterTransactionsBasedOnMonth(transactions, selectedMonth);
              let currMonthCatTxns: Transaction[] = [];
              if (this.helperService.isCategoryCreditCard(category)) {
                currMonthCatTxns = this.helperService.getTransactionsForAccount(currMonthCatTxns, [
                  ...ccAccounts.map((acc) => acc.id!),
                ]);
              } else {
                currMonthCatTxns = this.helperService.getTransactionsForCategory(currentMonthTxns, [category.id!]);
              }
              if (category?.budgeted?.[selectedMonth] === undefined) {
                category.budgeted = {
                  ...category.budgeted,
                  [selectedMonth]: 0,
                };
              }
              category.activity = {
                ...category.activity,
                [selectedMonth]: currMonthCatTxns.reduce((acc, curr) => acc + curr.amount, 0),
              };
              category.balance = {
                ...category.balance,
                [selectedMonth]: this.helperService.getCategoryBalance(selectedMonth, category, currMonthCatTxns),
              };
              return { ...category, showBudgetInput: false };
            }),
          ],
        };
        categoryGroupData.push(data);
      }
    }
    ctx.setState({
      ...ctx.getState(),
      categoryGroups: categoryGroupData,
    });
  }
}
