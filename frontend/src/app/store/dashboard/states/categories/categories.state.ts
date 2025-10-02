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
import { HttpService } from 'src/app/services/http.service';

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
  BASE_ENDPOINT = 'categories';
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
      const foundCategory = state.allCategories.find((cat) => cat.id === id) ?? null;
      return JSON.parse(JSON.stringify(foundCategory));
    });
  }
  static getCategoryFromName(name: string): (state: CategoriesStateModel) => Category | null {
    return createSelector([CategoriesState], (state: CategoriesStateModel) => {
      const foundCategory = state.allCategories.find((cat) => cat.name === name) ?? null;
      return JSON.parse(JSON.stringify(foundCategory));
    });
  }

  constructor(
    private ngxsStore: Store,
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private categoriesFs: CategoriesFirestore,
    private helperService: HelperService,
    private httpService: HttpService,
  ) { }

  // @Action(CategoriesActions.GetAllCategories)
  // initCategoriesStream(ctx: StateContext<CategoriesStateModel>, { budgetId }: CategoriesActions.GetAllCategories) {
  //   this.ngxsFirestoreConnect.connect(CategoriesActions.GetAllCategories, {
  //     to: () => this.categoriesFs.collection$((ref) => query(ref, where('budgetId', '==', budgetId))),
  //     connectedActionFinishesOn: 'FirstEmit',
  //   });
  // }
  //
  // @Action(StreamEmitted(CategoriesActions.GetAllCategories))
  // getAllCategories(
  //   ctx: StateContext<CategoriesStateModel>,
  //   { payload }: Emitted<CategoriesActions.GetAllCategories, Category[]>,
  // ) {
  //   console.log("CATEGORIES::::", payload);
  //   ctx.setState({
  //     allCategories: payload,
  //     inflowCategory: (payload.find((cat) => cat.name === INFLOW_CATEGORY_NAME) as unknown as InflowCategory) ?? null,
  //   });
  // }

  @Action(CategoriesActions.GetCategories)
  getCategories(ctx: StateContext<CategoriesStateModel>) {
    this.httpService.get<Category[]>(this.BASE_ENDPOINT).subscribe({
      next: (categories) => {
        ctx.setState({
          allCategories: categories,
          inflowCategory: (categories.find((cat) => cat.name === INFLOW_CATEGORY_NAME) as unknown as InflowCategory) ?? null,
        })
      }
    });
  }

  @Action(CategoriesActions.CreateCategory)
  createCategory(ctx: StateContext<CategoriesStateModel>, { payload }: CategoriesActions.CreateCategory) {
    this.httpService.post<Category>(this.BASE_ENDPOINT, payload).subscribe({
      next: (res) => {},
      error: (err) => {},
    })
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

  @Action(CategoriesActions.UpdateCategoryBudgeted)
  updateCategoryBudgeted(ctx: StateContext<CategoriesStateModel>, { payload }: CategoriesActions.UpdateCategoryBudgeted) {
    this.httpService
      .patch(`${this.BASE_ENDPOINT}/${payload.categoryId}/${payload.month}`, { budgeted: payload.budgeted })
      .subscribe({
        next: (res) => {
          console.log('budget updated')
          this.ngxsStore.dispatch(new CategoriesActions.SetInflowCategoryBalance())
        },
        error: (err: any) => {
          console.log(err)
        }
      })
  }

  // @Action(CategoriesActions.SetInflowCategoryBalance)
  // setInflowCategoryBalance(ctx: StateContext<CategoriesStateModel>) {
  //   console.log('setInflowCategoryBalance', ctx);
  //   const state = ctx.getState();
  //   const inflowCategory = Object.assign({}, state.inflowCategory);
  //   console.log(inflowCategory);
  //   const transactions = this.ngxsStore.selectSnapshot(TransactionsState.getAllTransactions);
  //   const categoriesWithoutInflow = state.allCategories.filter((cat) => cat.name !== INFLOW_CATEGORY_NAME);
  //   console.log(categoriesWithoutInflow);
  //   if (inflowCategory) {
  //     const totalBudgeted = categoriesWithoutInflow.reduce((totalBudgeted, cat) => {
  //       return totalBudgeted + Object.values(cat.budgeted).reduce((a, b) => a + b, 0);
  //     }, 0);
  //     console.log('totalBudgeted:', totalBudgeted);
  //     const inflowAmount = this.helperService
  //       .getTransactionsForCategory(transactions, [inflowCategory.id!])
  //       .reduce((totalAmount, transaction) => totalAmount + transaction.amount, 0);
  //     console.log('inflowAmount:', inflowAmount);
  //     inflowCategory.budgeted = Number(Number(inflowAmount - totalBudgeted).toFixed(2));
  //
  //     ctx.patchState({
  //       inflowCategory,
  //     });
  //   }
  // }

  @Action(CategoriesActions.SetInflowCategoryBalance)
  setInflowCategoryBalance(ctx: StateContext<CategoriesStateModel>) {
    this.httpService.get<number>(`${this.BASE_ENDPOINT}/inflow`).subscribe({
      next: (balance) => {
        const inflowCategory = Object.assign({}, ctx.getState().inflowCategory);
        if (inflowCategory) {
          inflowCategory.budgeted = balance;
          ctx.patchState({
            inflowCategory,
          });
        }
      }
    });
  }
}
