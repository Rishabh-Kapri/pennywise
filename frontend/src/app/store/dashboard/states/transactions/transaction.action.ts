export namespace TransactionsActions {
  export class GetAllTransactions {
    static readonly type = '[Transactions] GetAllTransactions';
    constructor(readonly budgetId: string) {}
  }
  export class ProcessNormalisedTransaction {
    static readonly type = '[Transactions] ProcessNormalisedTransactions';
  }
}
