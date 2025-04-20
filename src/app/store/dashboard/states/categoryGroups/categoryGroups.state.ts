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
import { query, where } from 'firebase/firestore';
import { StateOperator, compose, patch, updateItem } from '@ngxs/store/operators';

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
  static getAllCategoryGroups(state: CategoryGroupsStateModel): CategoryGroup[] {
    return state.allCategoryGroups;
  }
  @Selector()
  static getCategoryGroupData(state: CategoryGroupsStateModel): CategoryGroupData[] {
    return state.categoryGroups;
  }
  @Selector()
  static getCollapseAllGroups(state: CategoryGroupsStateModel): boolean {
    return state.collapseAllGroups;
  }

  constructor(
    private ngxsStore: Store,
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private categoryGroupsFs: CategoryGroupsFirestore,
    private helperService: HelperService,
  ) {}

  ngxsOnInit(): void {}

  @Action(CategoryGroupsActions.GetAllCategoryGroups)
  initCategoryGroupsStream(
    ctx: StateContext<CategoryGroupsStateModel>,
    { budgetId }: CategoryGroupsActions.GetAllCategoryGroups,
  ) {
    this.ngxsFirestoreConnect.connect(CategoryGroupsActions.GetAllCategoryGroups, {
      to: () => this.categoryGroupsFs.collection$((ref) => query(ref, where('budgetId', '==', budgetId))),
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(CategoryGroupsActions.GetAllCategoryGroups))
  getAllBudgets(
    ctx: StateContext<CategoryGroupsStateModel>,
    { payload }: Emitted<CategoryGroupsActions.GetAllCategoryGroups, CategoryGroup[]>,
  ) {
    ctx.patchState({
      allCategoryGroups: payload,
    });
  }

  @Action(CategoryGroupsActions.ToggleCategoryGroupsCollapse)
  toggleCategoryGroupsCollapse(ctx: StateContext<CategoryGroupsStateModel>) {
    ctx.patchState({
      collapseAllGroups: !ctx.getState().collapseAllGroups,
    });
    const collapsed = { collapsed: ctx.getState().collapseAllGroups };
    const indexList = ctx.getState().categoryGroups.reduce((result, r, i) => (result.push(i), result), <number[]>[]);
    const updateItems = indexList.map((index) => updateItem(index, patch(collapsed)));
    ctx.setState(
      patch({
        categoryGroups: compose(...updateItems) as unknown as StateOperator<CategoryGroupData[]>,
      }),
    );
  }

  @Action(CategoryGroupsActions.ToggleCategoryGroupCollapse)
  toggleSingleCategoryGroupCollapse(
    ctx: StateContext<CategoryGroupsStateModel>,
    { payload }: CategoryGroupsActions.ToggleCategoryGroupCollapse,
  ) {
    const categoryGroup = {
      ...payload,
      collapsed: !payload.collapsed,
    };
    ctx.setState(
      patch<CategoryGroupsStateModel>({
        categoryGroups: updateItem<CategoryGroupData>((group) => group.id === payload.id, categoryGroup),
      }),
    );
  }

  @Action(CategoryGroupsActions.UpdateCategoryInGroup)
  updateCategoryInGroup(
    ctx: StateContext<CategoryGroupsStateModel>,
    { groupId, categoryId, data }: CategoryGroupsActions.UpdateCategoryInGroup,
  ) {
    ctx.setState(
      patch<CategoryGroupsStateModel>({
        categoryGroups: updateItem<CategoryGroupData>(
          (group) => !!group && group.id === groupId,
          patch({
            categories: updateItem((cat) => !!cat && cat.id === categoryId, patch(data)),
          }),
        ),
      }),
    );
  }

  @Action(CategoryGroupsActions.SetCategoryGroupData)
  setCategoryGroupData(ctx: StateContext<CategoryGroupsStateModel>) {
    const categories = <Category[]>(
      JSON.parse(JSON.stringify(this.ngxsStore.selectSnapshot(CategoriesState.getAllCategories)))
    );
    const transactions = this.ngxsStore.selectSnapshot(TransactionsState.getAllTransactions);
    const selectedMonth = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth);
    const ccAccounts = this.ngxsStore.selectSnapshot(AccountsState.getCreditCardAccounts);
    const state = ctx.getState();

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
    const hiddenCategories = categories.filter((cat) => cat.hidden);
    const hiddenGroup = {
      name: 'Hidden',
      id: `hidden-cat`,
      balance: {
        [selectedMonth]: categories.reduce((amount, cat) => {
          return amount + (cat?.balance?.[selectedMonth] ?? 0);
        }, 0),
      },
      activity: {
        [selectedMonth]: categories.reduce((amount, cat) => {
          return amount + (cat?.activity?.[selectedMonth] ?? 0);
        }, 0),
      },
      budgeted: {
        [selectedMonth]: categories.reduce((amount, cat) => {
          return amount + (cat?.budgeted?.[selectedMonth] ?? 0);
        }, 0),
      },
      collapsed: true,
      categories: [
        ...hiddenCategories.map((category) => {
          const currentMonthTransactions = this.helperService.filterTransactionsBasedOnMonth(
            transactions,
            selectedMonth,
          );
          let currMonthCatTransactions: Transaction[] = [];
          if (this.helperService.isCategoryCreditCard(category)) {
            currMonthCatTransactions = this.helperService.getTransactionsForAccount(currentMonthTransactions, [
              ...ccAccounts.map((acc) => acc.id!),
            ]);
          } else {
            currMonthCatTransactions = this.helperService.getTransactionsForCategory(currentMonthTransactions, [
              category.id!,
            ]);
          }
          if (category?.budgeted?.[selectedMonth] === undefined) {
            category.budgeted = { ...category.budgeted, [selectedMonth]: 0 };
          }
          category.activity = {
            ...category.activity,
            [selectedMonth]: currMonthCatTransactions.reduce((acc, curr) => acc + curr.amount, 0),
          };
          // category.balance = this.getCategoryBalanceIterative(
          //   this.selectedMonth,
          //   category,
          //   currMonthCatTransactions
          // );
          category.balance = {
            ...category.balance,
            [selectedMonth]: this.helperService.getCategoryBalance(selectedMonth, category, currMonthCatTransactions),
          };
          return { ...category, showBudgetInput: false };
        }),
      ],
    };
    categoryGroupData.push(hiddenGroup);
    ctx.patchState({
      categoryGroups: categoryGroupData,
    });
  }
}
