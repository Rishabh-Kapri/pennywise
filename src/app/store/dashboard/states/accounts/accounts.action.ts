import { Account } from 'src/app/models/account.model';
import { AccountStateModel } from './accounts.state';

export namespace AccountsActions {
  export class GetAllAccounts {
    static readonly type = '[Accounts] GetAll';
  }
  export class Get {
    static readonly type = '[Accounts] Get';
    constructor(readonly payload: Account[]) {}
  }
  export class SetAllAccounts {
    static readonly type = '[SetAllAccounts] action';
    constructor(readonly payload: Account[]) {}
  }
  export class SetBudgetAccounts {
    static readonly type = '[SetBudgetAccounts] action';
    constructor(readonly payload: Account[]) {}
  }
  export class SetTrackingAccounts {
    static readonly type = '[SetTrackingAccounts] action';
    constructor(readonly payload: Account[]) {}
  }
}

