import { Account } from 'src/app/models/account.model';

export namespace AccountsActions {
  export class GetAllAccounts {
    static readonly type = '[Accounts] GetAllAccounts';
    constructor(readonly budgetId: string) {}
  }
  export class GetAccounts {
    static readonly type =  '[Accounts] GetAccounts';
  }
  export class SetBudgetAccounts {
    static readonly type = '[Accounts] SetBudgetAccounts';
  }
  export class SetTrackingAccounts {
    static readonly type = '[Accounts] SetTrackingAccounts';
  }
  export class SetSelectedAccount {
    static readonly type = '[Account] SetSelectedAccount';
    constructor(readonly payload: Account | null) {}
  }
  export class SetBalanceForAccounts {
    static readonly type = '[Account] SetBalanceForAccounts';
  }
}
