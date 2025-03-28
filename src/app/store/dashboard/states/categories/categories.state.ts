import { Injectable } from '@angular/core';
import { Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { Action, NgxsOnInit, Selector, State, StateContext } from '@ngxs/store';
import { Category } from 'src/app/models/category.model';
import { CategoriesFirestore } from 'src/app/services/databases/categories.firestore';
import { CategoriesActions } from './categories.action';

export interface CategoriesStateModel {
  allCategories: Category[];
}
@State<CategoriesStateModel>({
  name: 'categories',
  defaults: {
    allCategories: [],
  },
})
@Injectable()
export class CategoriesState implements NgxsOnInit {
  @Selector()
  static getAllCategories(state: CategoriesStateModel): Category[] {
    return state.allCategories;
  }

  constructor(
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private categoriesFs: CategoriesFirestore,
  ) {}

  ngxsOnInit(ctx: StateContext<any>): void {
    this.ngxsFirestoreConnect.connect(CategoriesActions.GetAllCategories, {
      to: () => this.categoriesFs.collection$(),
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(CategoriesActions.GetAllCategories))
  getAllBudgets(
    ctx: StateContext<CategoriesStateModel>,
    { action, payload }: Emitted<CategoriesActions.GetAllCategories, Category[]>,
  ) {
    ctx.setState({
      allCategories: payload,
    });
  }
}
