import { Transaction } from 'src/app/models/transaction.model';

export namespace TransactionsActions {
  export class GetAllTransactions {
    static readonly type = '[Transactions] GetAllTransactions';
    constructor(readonly budgetId: string) { }
  }
  export class GetNormalisedTransaction {
    static readonly type = '[Transactions] GetNormalisedTransactions';
    constructor(readonly accountId: string = '') { }
  }
  export class ProcessNormalisedTransaction {
    static readonly type = '[Transactions] ProcessNormalisedTransactions';
  }
  export class CreateTransaction {
    static readonly type = '[Transactions] CreateTransaction';
    constructor(readonly payload: Partial<Transaction>) { }
  }
  export class EditTransaction {
    static readonly type = '[Transactions] EditTransaction';
    constructor(readonly data: Partial<Transaction>) { }
  }
  export class DeleteTransaction {
    static readonly type = '[Transaction] DeleteTransaction';
    constructor(readonly txnId: string) { }
  }
}
