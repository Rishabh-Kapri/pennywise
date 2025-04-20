import { Injectable } from '@angular/core';
import { Emitted, NgxsFirestoreConnect, NgxsFirestorePageService, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { Action, NgxsOnInit, Selector, State, StateContext, Store } from '@ngxs/store';
import { NormalizedTransaction } from 'src/app/models/transaction.model';
import { TransactionsFirestore } from 'src/app/services/databases/transactions.firestore';
import { TransactionsActions } from './transaction.action';
import { Transaction } from 'src/app/models/transaction.model';
import { AccountsState } from '../accounts/accounts.state';
import { PayeesState } from '../payees/payees.state';
import { CategoriesState } from '../categories/categories.state';
import { and, orderBy, query, where } from 'firebase/firestore';
import { ConfigState } from '../config/config.state';

export interface TransactionsStateModel {
  allTransactions: Transaction[];
  normalizedTransactions: NormalizedTransaction[];
}
@State<TransactionsStateModel>({
  name: 'transactions',
  defaults: {
    allTransactions: [],
    normalizedTransactions: [],
  },
})
@Injectable()
export class TransactionsState implements NgxsOnInit {
  @Selector()
  static getAllTransactions(state: TransactionsStateModel): Transaction[] {
    return state.allTransactions;
  }
  @Selector()
  static getNormalizedTransaction(state: TransactionsStateModel): NormalizedTransaction[] {
    return state.normalizedTransactions;
  }

  constructor(
    private ngxsStore: Store,
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private transactionsFs: TransactionsFirestore,
  ) {}

  ngxsOnInit(): void {}

  @Action(TransactionsActions.GetAllTransactions)
  initTransactionsStream(
    ctx: StateContext<TransactionsStateModel>,
    { budgetId }: TransactionsActions.GetAllTransactions,
  ) {
    this.ngxsFirestoreConnect.connect(TransactionsActions.GetAllTransactions, {
      to: () => {
        return this.transactionsFs.collection$((ref) =>
          query(
            ref,
            and(where('budgetId', '==', budgetId), where('deleted', '==', false)),
            orderBy('date', 'desc'),
            orderBy('updatedAt', 'desc'),
          ),
        );
      },
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(TransactionsActions.GetAllTransactions))
  getAllTransactions(
    ctx: StateContext<TransactionsStateModel>,
    { payload }: Emitted<TransactionsActions.GetAllTransactions, Transaction[]>,
  ) {
    ctx.patchState({
      allTransactions: payload,
    });
    const isStateLoading = this.ngxsStore.selectSnapshot(ConfigState.getStateLoadingStatus);
    // processNormalisedTransactions if state is finished loading
    if (!isStateLoading) {
      this.ngxsStore.dispatch(new TransactionsActions.ProcessNormalisedTransaction());
    }
  }

  @Action(TransactionsActions.ProcessNormalisedTransaction)
  processNormalisedTransactions(ctx: StateContext<TransactionsStateModel>) {
    // @TODO:
    // process only few transactions and rest on scroll
    const allAccounts = this.ngxsStore.selectSnapshot(AccountsState.getAllAccounts);
    const selectedAccount = this.ngxsStore.selectSnapshot(AccountsState.getSelectedAccount);
    const allPayees = this.ngxsStore.selectSnapshot(PayeesState.getAllPayees);
    const allCategories = this.ngxsStore.selectSnapshot(CategoriesState.getAllCategories);
    const state = ctx.getState();

    const normalizedTransactions: NormalizedTransaction[] = [];
    let prevTransacAmount = 0;
    let accBal = 0;
    const transactions = state.allTransactions.filter(
      (txn) => !selectedAccount || txn.accountId === selectedAccount.id,
    );
    for (const [index, value] of transactions.entries()) {
      const transaction = value;
      const account = allAccounts.find((acc) => acc.id === transaction.accountId);
      if (index > 0) {
        accBal = normalizedTransactions[index - 1].balance - prevTransacAmount;
      } else {
        accBal = account?.balance ?? 0;
      }
      const payee = allPayees.find((payee) => payee.id === transaction.payeeId);
      const normalizedTxn: NormalizedTransaction = {
        id: transaction.id!,
        budgetId: transaction.budgetId,
        date: transaction.date,
        outflow: transaction.amount < 0 ? Math.abs(transaction.amount) : null,
        inflow: transaction.amount >= 0 ? Math.abs(transaction.amount) : null,
        balance: Number(accBal.toFixed(2)),
        note: transaction.note,
        transferTransactionId: transaction.transferTransactionId ?? null,
        transferAccountId: transaction.transferAccountId ?? null,
        accountName: account?.name ?? '',
        accountId: account?.id!,
        payeeName: payee?.name ?? '',
        payeeId: payee?.id!,
        categoryName: allCategories.find((cat) => cat.id === transaction.categoryId)?.name ?? null,
        categoryId: transaction.categoryId,
      };
      normalizedTransactions.push(normalizedTxn);
      prevTransacAmount = transaction.amount;
    }
    ctx.patchState({
      normalizedTransactions,
    });
  }
}
