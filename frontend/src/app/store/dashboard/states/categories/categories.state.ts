import { Injectable } from '@angular/core';
import { Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { Action, Selector, State, StateContext, Store, createSelector } from '@ngxs/store';
import { Category, InflowCategory } from 'src/app/models/category.model';
import { CategoriesFirestore } from 'src/app/services/databases/categories.firestore';
import { CategoriesActions } from './categories.action';
import { INFLOW_CATEGORY_NAME } from 'src/app/constants/general';
import { HelperService } from 'src/app/services/helper.service';
import { TransactionsState } from '../transactions/transactions.state';
import { query, where } from 'firebase/firestore';
import { CategoryGroupsActions } from '../categoryGroups/categoryGroups.action';
import { patch, updateItem } from '@ngxs/store/operators';

export interface CategoriesStateModel {
  allCategories: Category[];
  inflowCategory: InflowCategory | null;
}
@State<CategoriesStateModel>({
  name: 'categories',
  defaults: {
    allCategories: [],
    inflowCategory: null,
  },
})
@Injectable()
export class CategoriesState {
  @Selector()
  static categories(state: CategoriesStateModel) {
    return state;
  }
  @Selector()
  static getAllCategories(state: CategoriesStateModel): Category[] {
    return state.allCategories;
  }
  @Selector()
  static getInflowWithBalance(state: CategoriesStateModel): InflowCategory | null {
    return state.inflowCategory;
  }

  static getCategory(id: string): (state: CategoriesStateModel) => Category | null {
    return createSelector([CategoriesState], (state: CategoriesStateModel) => {
      const foundCategory = state.allCategories.find(cat => cat.id === id) ?? null;
      return JSON.parse(JSON.stringify(foundCategory));
    });
  }
  static getCategoryFromName(name: string): (state: CategoriesStateModel) => Category | null {
    return createSelector([CategoriesState], (state: CategoriesStateModel) => {
      const foundCategory = state.allCategories.find(cat => cat.name === name) ?? null;
      return JSON.parse(JSON.stringify(foundCategory));
    });
  }

  constructor(
    private ngxsStore: Store,
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private categoriesFs: CategoriesFirestore,
    private helperService: HelperService,
  ) {}

  @Action(CategoriesActions.GetAllCategories)
  initCategoriesStream(ctx: StateContext<CategoriesStateModel>, { budgetId }: CategoriesActions.GetAllCategories) {
    this.ngxsFirestoreConnect.connect(CategoriesActions.GetAllCategories, {
      to: () => this.categoriesFs.collection$((ref) => query(ref, where('budgetId', '==', budgetId))),
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(CategoriesActions.GetAllCategories))
  getAllCategories(
    ctx: StateContext<CategoriesStateModel>,
    { payload }: Emitted<CategoriesActions.GetAllCategories, Category[]>,
  ) {
    ctx.setState({
      allCategories: payload,
      inflowCategory: (payload.find((cat) => cat.name === INFLOW_CATEGORY_NAME) as unknown as InflowCategory) ?? null,
    });
  }

  @Action(CategoriesActions.UpdateCategory)
  updateCategory(ctx: StateContext<CategoriesStateModel>, { payload }: CategoriesActions.UpdateCategory) {
    ctx.setState(
      patch<CategoriesStateModel>({
        allCategories: updateItem<Category>((cat) => {
          return cat.id === payload.id;
        }, payload),
      }),
    );
  }

  @Action(CategoriesActions.SetInflowCategoryBalance)
  setInflowCategoryBalance(ctx: StateContext<CategoriesStateModel>) {
    console.log('setInflowCategoryBalance', ctx);
    const state = ctx.getState();
    const inflowCategory = Object.assign({}, state.inflowCategory);
    console.log(inflowCategory);
    const transactions = this.ngxsStore.selectSnapshot(TransactionsState.getAllTransactions);
    const categoriesWithoutInflow = state.allCategories.filter((cat) => cat.name !== INFLOW_CATEGORY_NAME);
    console.log(categoriesWithoutInflow);
    if (inflowCategory) {
      const totalBudgeted = categoriesWithoutInflow.reduce((totalBudgeted, cat) => {
        return totalBudgeted + Object.values(cat.budgeted).reduce((a, b) => a + b, 0);
      }, 0);
      console.log('totalBudgeted:', totalBudgeted);
      const inflowAmount = this.helperService
        .getTransactionsForCategory(transactions, [inflowCategory.id!])
        .reduce((totalAmount, transaction) => totalAmount + transaction.amount, 0);
      console.log('inflowAmount:', inflowAmount);
      inflowCategory.budgeted = Number(Number(inflowAmount - totalBudgeted).toFixed(2));

      ctx.patchState({
        inflowCategory,
      });
    }
  }
}
