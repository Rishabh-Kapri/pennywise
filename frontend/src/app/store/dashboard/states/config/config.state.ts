import { Injectable } from '@angular/core';
import { Action, Selector, State, StateContext, Store } from '@ngxs/store';
import { ConfigActions } from './config.action';
import { AccountsActions } from '../accounts/accounts.action';
import { CategoryGroupsActions } from '../categoryGroups/categoryGroups.action';
import { TransactionsActions } from '../transactions/transaction.action';
import { CategoriesActions } from '../categories/categories.action';
import { SelectedComponent } from 'src/app/models/state.model';

export interface ConfigStateModel {
  selectedMonth: string;
  selectedComponent: SelectedComponent;
  isStateLoading: boolean;
}
@State<ConfigStateModel>({
  name: 'config',
  defaults: {
    selectedMonth: '',
    selectedComponent: SelectedComponent.REPORTS,
    isStateLoading: true,
  },
})
@Injectable()
export class ConfigState {
  @Selector()
  static getStateLoadingStatus(state: ConfigStateModel): boolean {
    return state.isStateLoading;
  }
  @Selector()
  static getSelectedComponent(state: ConfigStateModel): SelectedComponent {
    return state.selectedComponent;
  }

  constructor(private ngxsStore: Store) {}

  @Action(ConfigActions.SetStateLoadingStatus)
  stateLoaded(ctx: StateContext<ConfigStateModel>, { payload }: ConfigActions.SetStateLoadingStatus) {
    ctx.patchState({
      isStateLoading: payload,
    });
    if (!payload) {
      // if state has been loaded
      this.ngxsStore.dispatch(new AccountsActions.SetBalanceForAccounts());
      this.ngxsStore.dispatch(new CategoriesActions.SetInflowCategoryBalance());
      this.ngxsStore.dispatch(new CategoryGroupsActions.SetCategoryGroupData());
    }
  }

  @Action(ConfigActions.SetSelectedComponent)
  setSelectedComponent(ctx: StateContext<ConfigStateModel>, { payload }: ConfigActions.SetSelectedComponent) {
    ctx.patchState({
      selectedComponent: payload,
    });
  }
}
