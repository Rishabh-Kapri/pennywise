import { Transaction } from 'firebase/firestore';

export namespace TransactionsActions {
  export class GetAllTransactions {
    static readonly type = '[Transactions] GetAll';
  }
  export class ProcessNormalisedTransaction {
    static readonly type = '[Transactions] ProcessNormalised';
  }
}
