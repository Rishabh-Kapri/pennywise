import { Injectable } from '@angular/core';
import { Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { Action, NgxsOnInit, Selector, State, StateContext } from '@ngxs/store';
import { NormalizedTransaction } from 'src/app/models/transaction.model';
import { TransactionsFirestore } from 'src/app/services/databases/transactions.firestore';
import { TransactionsActions } from './transaction.action';
import { Transaction } from 'src/app/models/transaction.model';

export interface TransactionsStateModel {
  allTransactions: Transaction[];
  normalisedTransactions: NormalizedTransaction[];
}
@State<TransactionsStateModel>({
  name: 'transactions',
  defaults: {
    allTransactions: [],
    normalisedTransactions: [],
  },
})
@Injectable()
export class TransactionsState implements NgxsOnInit {
  @Selector()
  static getAllTransactions(state: TransactionsStateModel): Transaction[] {
    return state.allTransactions;
  }

  constructor(
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private transactionsFs: TransactionsFirestore,
  ) {}

  ngxsOnInit(ctx: StateContext<any>): void {
    this.ngxsFirestoreConnect.connect(TransactionsActions.GetAllTransactions, {
      to: () => this.transactionsFs.collection$(),
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(TransactionsActions.GetAllTransactions))
  getAllBudgets(
    ctx: StateContext<TransactionsStateModel>,
    { action, payload }: Emitted<TransactionsActions.GetAllTransactions, Transaction[]>,
  ) {
    ctx.setState({
      ...ctx.getState(),
      allTransactions: payload,
    });
  }

  @Action(TransactionsActions.ProcessNormalisedTransaction)
  processNormalisedTransactions(ctx: StateContext<TransactionsStateModel>) {
    console.log('processNormalisedTransactions:', ctx);
  }
}
