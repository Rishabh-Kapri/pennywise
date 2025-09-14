import { Payee } from 'src/app/models/payee.model';

export namespace PayeesActions {
  export class GetPayees {
    static readonly type = '[Payees] GetPayees';
  }
  export class GetAllPayees {
    static readonly type = '[Payees] GetAllPayees';
    constructor(readonly budgetId: string) {}
  }
  export class CreatePayee {
    static readonly type = '[Payees] CreatePayee';
    constructor(readonly payload: Partial<Payee>) {}
  }
}
