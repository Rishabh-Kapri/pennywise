import { Injectable } from '@angular/core';
import { Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { Action, NgxsOnInit, Selector, State, StateContext } from '@ngxs/store';
import { Budget } from 'src/app/models/budget.model';
import { BudgetsFirestore } from 'src/app/services/databases/budgets.firestore';
import { BudgetsActions } from './budget.action';
import { HelperService } from 'src/app/services/helper.service';

export interface BudgetsStateModel {
  allBudgets: Budget[];
  selectedBudget: Budget | null;
  selectedMonth: string;
}
@State<BudgetsStateModel>({
  name: 'budgets',
  defaults: {
    allBudgets: [],
    selectedBudget: null,
    selectedMonth: '',
  },
})
@Injectable()
export class BudgetsState implements NgxsOnInit {
  @Selector()
  static getSelectedMonth(state: BudgetsStateModel): string {
    return state.selectedMonth;
  }
  @Selector()
  static getAllBudgets(state: BudgetsStateModel): Budget[] {
    return state.allBudgets;
  }
  @Selector()
  static getSelectedBudget(state: BudgetsStateModel): Budget | null {
    return state.selectedBudget;
  }

  constructor(
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private budgetsFs: BudgetsFirestore,
    private helperService: HelperService,
  ) {}

  ngxsOnInit(ctx: StateContext<any>): void {
    this.ngxsFirestoreConnect.connect(BudgetsActions.GetAllBudgets, {
      to: () => this.budgetsFs.collection$(),
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(BudgetsActions.GetAllBudgets))
  getAllBudgets(
    ctx: StateContext<BudgetsStateModel>,
    { action, payload }: Emitted<BudgetsActions.GetAllBudgets, Budget[]>,
  ) {
    ctx.setState({
      allBudgets: payload,
      selectedBudget: payload.find((budget) => budget.isSelected === true) ?? null,
      selectedMonth: this.helperService.getCurrentMonthKey(),
    });
  }
}
