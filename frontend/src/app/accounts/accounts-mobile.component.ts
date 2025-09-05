import { Component, OnInit } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { Store } from '@ngxs/store';
import { Account } from '../models/account.model';
import { AccountsState } from '../store/dashboard/states/accounts/accounts.state';

@Component({
  selector: 'app-accounts-mobile',
  templateUrl: './accounts-mobile.component.html',
  styleUrls: ['./accounts-mobile.component.scss']
})
export class AccountsMobileComponent implements OnInit {
  budgetAccountData$: Observable<{ totalAmount: number; accounts: Account[] }>;
  trackingAccountData$: Observable<{ totalAmount: number; accounts: Account[] }>;
  closedAccountData$: Observable<{ totalAmount: number; accounts: Account[] }>;

  showAccountModal = false;
  editingAccount: Account | null = null;

  constructor(private ngxsStore: Store) {}

  ngOnInit(): void {
    this.budgetAccountData$ = this.ngxsStore.select(AccountsState.getBudgetAccounts).pipe(
      map(accounts => ({
        totalAmount: Number(accounts.reduce((a, b) => a + (b.balance ?? 0), 0).toFixed(2)),
        accounts
      }))
    );
    this.trackingAccountData$ = this.ngxsStore.select(AccountsState.getTrackingAccounts).pipe(
      map(accounts => ({
        totalAmount: Number(accounts.reduce((a, b) => a + (b.balance ?? 0), 0).toFixed(2)),
        accounts
      }))
    );
    this.closedAccountData$ = this.ngxsStore.select(AccountsState.getClosedAccounts).pipe(
      map(accounts => ({
        totalAmount: Number(accounts.reduce((a, b) => a + (b.balance ?? 0), 0).toFixed(2)),
        accounts
      }))
    );
  }

  openAddAccount() {
    this.editingAccount = null;
    this.showAccountModal = true;
  }

  openEditAccount(account: Account) {
    this.editingAccount = account;
    this.showAccountModal = true;
  }

  closeAccountModal() {
    this.showAccountModal = false;
    this.editingAccount = null;
  }
} 
