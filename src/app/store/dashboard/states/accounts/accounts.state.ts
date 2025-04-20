import { Injectable } from '@angular/core';
import { Action, Selector, State, StateContext, Store } from '@ngxs/store';
import { AccountsActions } from './accounts.action';
import { Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { AccountsFirestore } from 'src/app/services/databases/accounts.firestore';
import { Account, BudgetAccountType, TrackingAccountType } from 'src/app/models/account.model';
import { TransactionsState } from '../transactions/transactions.state';
import { query, where } from 'firebase/firestore';
import { ConfigActions } from '../config/config.action';
import { TransactionsActions } from '../transactions/transaction.action';

export interface AccountStateModel {
  selectedAccount: Account | null;
  allAccounts: Account[];
  trackingAccounts: Account[];
  budgetAccounts: Account[];
}
@State<AccountStateModel>({
  name: 'accounts',
  defaults: {
    selectedAccount: null,
    allAccounts: [],
    trackingAccounts: [],
    budgetAccounts: [],
  },
})
@Injectable()
export class AccountsState {
  private filterTrackingAccounts(accounts: Account[]) {
    return accounts.filter(
      (acc) =>
        [TrackingAccountType.ASSET, TrackingAccountType.LIABILITY].includes(<TrackingAccountType>acc.type) &&
        !acc.closed,
    );
  }
  private filterBudgetAccounts(accounts: Account[]) {
    return accounts.filter(
      (acc) =>
        [BudgetAccountType.SAVINGS, BudgetAccountType.CHECKING, BudgetAccountType.CREDIT_CARD].includes(
          <BudgetAccountType>acc.type,
        ) && !acc.closed,
    );
  }
  @Selector()
  static getSelectedAccount(state: AccountStateModel): Account | null {
    return state.selectedAccount;
  }
  @Selector()
  static getAllAccounts(state: AccountStateModel): Account[] {
    return state.allAccounts;
  }
  @Selector()
  static getClosedAccounts(state: AccountStateModel): Account[] {
    return state.allAccounts.filter((acc) => acc.closed);
  }
  @Selector()
  static getTrackingAccounts(state: AccountStateModel): Account[] {
    return state.trackingAccounts;
  }
  @Selector()
  static getBudgetAccounts(state: AccountStateModel): Account[] {
    return state.budgetAccounts;
  }
  @Selector()
  static getCreditCardAccounts(state: AccountStateModel): Account[] {
    return state.budgetAccounts.filter((acc) => acc.type === BudgetAccountType.CREDIT_CARD);
  }
  @Selector()
  static getTotalCurrentFunds(state: AccountStateModel): number {
    return state.allAccounts.reduce((a, b) => a + b.balance, 0);
  }

  constructor(
    private ngxsStore: Store,
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private accountsFs: AccountsFirestore,
  ) {}

  @Action(AccountsActions.GetAllAccounts)
  initAccountsStream(ctx: StateContext<AccountStateModel>, { budgetId }: AccountsActions.GetAllAccounts) {
    this.ngxsFirestoreConnect.connect(AccountsActions.GetAllAccounts, {
      to: () => this.accountsFs.collection$((ref) => query(ref, where('budgetId', '==', budgetId))),
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(AccountsActions.GetAllAccounts))
  getAllAccounts(
    ctx: StateContext<AccountStateModel>,
    { payload }: Emitted<AccountsActions.GetAllAccounts, Account[]>,
  ) {
    ctx.patchState({
      allAccounts: payload,
      budgetAccounts: this.filterBudgetAccounts(payload),
      trackingAccounts: this.filterTrackingAccounts(payload),
    });
    this.ngxsStore.dispatch(new ConfigActions.SetStateLoadingStatus(false));
  }

  @Action(AccountsActions.SetBudgetAccounts)
  setBudgetAccounts(ctx: StateContext<AccountStateModel>) {
    ctx.patchState({
      budgetAccounts: this.filterBudgetAccounts(ctx.getState().allAccounts),
    });
  }

  @Action(AccountsActions.SetTrackingAccounts)
  setTrackingAccounts(ctx: StateContext<AccountStateModel>) {
    ctx.patchState({
      trackingAccounts: this.filterTrackingAccounts(ctx.getState().allAccounts),
    });
  }

  @Action(AccountsActions.SetSelectedAccount)
  setSelectedAccount(ctx: StateContext<AccountStateModel>, { payload }: AccountsActions.SetSelectedAccount) {
    ctx.patchState({
      selectedAccount: payload,
    });
    this.ngxsStore.dispatch(new TransactionsActions.ProcessNormalisedTransaction());
  }

  @Action(AccountsActions.SetBalanceForAccounts)
  setBalanceForAccounts(ctx: StateContext<AccountStateModel>) {
    const state = ctx.getState();
    const accounts = <Account[]>JSON.parse(JSON.stringify(state.allAccounts));
    const transactions = this.ngxsStore.selectSnapshot(TransactionsState.getAllTransactions);

    for (const acc of accounts) {
      const amount = transactions.filter((txn) => txn.accountId === acc.id!).reduce((a, b) => a + b.amount, 0);
      acc.balance = Number(amount.toFixed(2));
    }
    ctx.patchState({
      allAccounts: accounts,
    });
    this.ngxsStore.dispatch(new AccountsActions.SetBudgetAccounts());
    this.ngxsStore.dispatch(new AccountsActions.SetTrackingAccounts());
  }
}
