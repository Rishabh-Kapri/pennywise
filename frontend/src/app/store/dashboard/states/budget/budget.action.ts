import { Budget } from 'src/app/models/budget.model';

export namespace BudgetsActions {
  export class GetAllBudgets {
    static readonly type = '[Budgets] GetAll';
  }
  export class SetSelectedMonth {
    static readonly type = '[Budgets] SetSelectedMonth';
    constructor(readonly payload: string) { }
  }
  export class SetSelectedBudget {
    static readonly type = '[Budgets] SetSelectedBudget';
    constructor(readonly payload: Budget) { }
  }
  export class BudgetsFetched {
    static readonly type = '[Budgets] BudgetsFetched';
  }
  export class CreateBudget {
    static readonly type = '[Budgets] CreateBudget';
    constructor(readonly payload: string) { }
  }
  export class UpdateBudget {
    static readonly type = '[Budgets] UpdateBudget';
    constructor(
      readonly payload: {
        id: string;
        name: string;
        isSelected: boolean;
      },
    ) { }
  }
}
