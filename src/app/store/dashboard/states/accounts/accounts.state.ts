import { Injectable } from '@angular/core';
import { Action, NgxsOnInit, Selector, State, StateContext } from '@ngxs/store';
import { AccountsActions } from './accounts.action';
import { Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { AccountsFirestore } from 'src/app/services/databases/accounts.firestore';
import { Account, BudgetAccountType, TrackingAccountType } from 'src/app/models/account.model';

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
export class AccountsState implements NgxsOnInit {
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

  constructor(
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private accountsFs: AccountsFirestore,
  ) {}

  ngxsOnInit(): void {
    console.log('ngxsOnInit');
    this.ngxsFirestoreConnect.connect(AccountsActions.GetAllAccounts, {
      to: () => this.accountsFs.collection$(),
      connectedActionFinishesOn: 'FirstEmit',
    });
    // this.ngxsFirestoreConnect.connect(AccountsActions.GetAllAccounts, {
    //   to: () => this.accountsFs.collection$(),
    // });
  }

  @Action(StreamEmitted(AccountsActions.GetAllAccounts))
  getAllAccounts(
    ctx: StateContext<AccountStateModel>,
    { action, payload }: Emitted<AccountsActions.GetAllAccounts, Account[]>,
  ) {
    console.log('getAllAccounts action');
    console.log(ctx, action, payload);
    ctx.setState({
      selectedAccount: null,
      allAccounts: payload,
      budgetAccounts: this.filterBudgetAccounts(payload),
      trackingAccounts: this.filterTrackingAccounts(payload),
    });
  }

  @Action(AccountsActions.SetAllAccounts)
  setAllAccounts(ctx: StateContext<Account[]>, { payload }: AccountsActions.SetAllAccounts) {
    ctx.setState(payload);
  }

  @Action(AccountsActions.SetBudgetAccounts)
  setBudgetAccounts(ctx: StateContext<Account[]>, { payload }: AccountsActions.SetAllAccounts) {
    ctx.setState(payload);
  }
  @Action(AccountsActions.SetTrackingAccounts)
  SetTrackingAccounts(ctx: StateContext<Account[]>, { payload }: AccountsActions.SetAllAccounts) {
    ctx.setState(payload);
  }
}
