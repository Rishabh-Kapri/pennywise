import { Injectable } from '@angular/core';
import { Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { Action, NgxsOnInit, Selector, State, StateContext, Store } from '@ngxs/store';
import { Budget } from 'src/app/models/budget.model';
import { BudgetsFirestore } from 'src/app/services/databases/budgets.firestore';
import { BudgetsActions } from './budget.action';
import { HelperService } from 'src/app/services/helper.service';
import { CategoryGroupsActions } from '../categoryGroups/categoryGroups.action';

export interface BudgetsStateModel {
  allBudgets: Budget[];
  selectedBudget: Budget | null;
  selectedMonth: string;
  selectedHumanMonth: string;
}
@State<BudgetsStateModel>({
  name: 'budgets',
  defaults: {
    allBudgets: [],
    selectedBudget: null,
    selectedMonth: '',
    selectedHumanMonth: '',
  },
})
@Injectable()
export class BudgetsState implements NgxsOnInit {
  @Selector()
  static getAllBudgets(state: BudgetsStateModel): Budget[] {
    return state.allBudgets;
  }
  @Selector()
  static getSelectedMonth(state: BudgetsStateModel): string {
    return state.selectedMonth;
  }
  @Selector()
  static getSelectedHumanMonth(state: BudgetsStateModel): string {
    return state.selectedHumanMonth;
  }
  @Selector()
  static getSelectedBudget(state: BudgetsStateModel): Budget | null {
    return state.selectedBudget;
  }
  @Selector()
  static getUnselectedBudgets(state: BudgetsStateModel): Budget[] {
    return state.allBudgets.filter((budget) => !budget.isSelected);
  }

  constructor(
    private ngxsStore: Store,
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private budgetsFs: BudgetsFirestore,
    private helperService: HelperService,
  ) {}

  ngxsOnInit(): void {
    this.ngxsFirestoreConnect.connect(BudgetsActions.GetAllBudgets, {
      to: () => this.budgetsFs.collection$(),
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(BudgetsActions.GetAllBudgets))
  getAllBudgets(ctx: StateContext<BudgetsStateModel>, { payload }: Emitted<BudgetsActions.GetAllBudgets, Budget[]>) {
    console.log("BUDGETS:::", payload);
    const selectedBudget = payload.find((budget) => budget.isSelected === true) ?? null;
    const selectedMonth = this.helperService.getCurrentMonthKey();
    ctx.setState({
      allBudgets: payload,
      selectedBudget,
      selectedMonth: selectedMonth,
      selectedHumanMonth: this.helperService.getSelectedMonthInHumanFormat(selectedMonth),
    });
    if (!selectedBudget) {
      // @TODO: dispatch an error message
    }
  }

  @Action(BudgetsActions.SetSelectedMonth)
  setSelectedMonth(ctx: StateContext<BudgetsStateModel>, { payload }: BudgetsActions.SetSelectedMonth) {
    ctx.patchState({
      selectedMonth: payload,
      selectedHumanMonth: this.helperService.getSelectedMonthInHumanFormat(payload),
    });
    this.ngxsStore.dispatch(new CategoryGroupsActions.SetCategoryGroupData());
  }

  @Action(BudgetsActions.SetSelectedBudget)
  setSelectedBudget(ctx: StateContext<BudgetsStateModel>, { payload }: BudgetsActions.SetSelectedBudget) {
    ctx.patchState({
      selectedBudget: payload,
    });
    // @TODO: dispatch all the fetch actions for transactions, accounts, etc again
  }
}
