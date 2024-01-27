import { AfterViewInit, Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { DatabaseService } from '../services/database.service';
import {
  Account,
  BudgetAccountType,
  TrackingAccountType,
  BudgetAccountNames,
  TrackingAccountNames,
} from '../models/account.model';
import { FormControl, FormGroup, Validators } from '@angular/forms';
import { StoreService } from '../services/store.service';
import { filter, map, switchMap } from 'rxjs/operators';
import { Observable, combineLatest, of } from 'rxjs';
import { Modal, ModalOptions } from 'flowbite';
import { SelectedComponent } from '../models/state.model';
import { Budget } from '../models/budget.model';
import { STARTING_BALANCE_PAYEE } from '../constants/general';

interface AccountForm {
  name: FormControl<string | null>;
  type: FormControl<BudgetAccountType | TrackingAccountType | null>;
  balance: FormControl<number | null>;
}

@Component({
  selector: 'app-sidebar',
  templateUrl: './sidebar.component.html',
  styleUrls: ['./sidebar.component.scss'],
})
export class SidebarComponent implements OnInit, AfterViewInit {
  @ViewChild('addAccountModal', { read: ElementRef }) addAccountModalRef: ElementRef<HTMLElement>;
  modalOptions: ModalOptions = {
    backdrop: 'dynamic',
    backdropClasses: 'bg-gray-900 bg-opacity-50 dark:bg-opacity-80 fixed inset-0 z-40',
    closable: true,
  };
  budgetAccountData$: Observable<{ totalAmount: number; accounts: Account[] }>;
  trackingAccountData$: Observable<{ totalAmount: number; accounts: Account[] }>;
  closedAccounts$: Observable<Account[]>;
  selectedComponent = SelectedComponent;
  text = 'Add';
  addAcountModal: Modal;
  accountForm: FormGroup<AccountForm>;
  budgetAccountNames = BudgetAccountNames;
  trackingAccountNames = TrackingAccountNames;

  editingAccount: Account;
  totalCurrentFunds$: Observable<number>;
  ynab_token = 'sDjhb_o63I9mPiRSW-x1Is_UNHSEZ4Uth4CbAR2Cayw';
  unSelectedBudgets$: Observable<Budget[]>;

  isLoading = false;
  newBudgetName = '';

  constructor(private dbService: DatabaseService, public store: StoreService) {}

  ngOnInit(): void {
    this.accountForm = new FormGroup<AccountForm>({
      name: new FormControl('', { validators: [Validators.required], nonNullable: true }),
      type: new FormControl(null, { validators: [Validators.required] }),
      balance: new FormControl(null, { validators: [Validators.required] }),
    });
    this.budgetAccountData$ = combineLatest([this.store.budgetAccounts$]).pipe(
      switchMap(([accounts]) => {
        let data = {
          totalAmount: accounts.reduce((a, b) => a + b.balance, 0),
          accounts,
        };
        return of(data);
      })
    );
    this.trackingAccountData$ = combineLatest([this.store.trackingAccounts$]).pipe(
      switchMap(([accounts]) => {
        let data = {
          totalAmount: accounts.reduce((a, b) => a + b.balance, 0),
          accounts,
        };
        return of(data);
      })
    );
    this.closedAccounts$ = this.store.allAccounts$.pipe(map((accounts) => accounts.filter((acc) => acc.closed)));
    this.totalCurrentFunds$ = this.store.allAccounts$?.pipe(map((data) => data.reduce((a, b) => a + b.balance, 0)));
    this.unSelectedBudgets$ = this.store.budget$.pipe(
      map((budgets) => budgets.filter((budget) => budget.isSelected === false))
    );
  }

  ngAfterViewInit(): void {
    this.addAcountModal = new Modal(this.addAccountModalRef.nativeElement, this.modalOptions);
  }

  resetAccountForm() {
    this.text = 'Add';
    this.accountForm.reset();
  }

  async addAccount() {
    this.text = 'Add';
    this.resetAccountForm();
  }

  async editAccount(account: Account) {
    this.editingAccount = account;
    this.text = 'Edit';
    this.accountForm.patchValue({
      name: account.name,
      type: account.type,
      balance: account.balance,
    });
    this.addAcountModal.toggle();
  }

  async submitAccount(form: FormGroup<AccountForm>) {
    if (form.invalid) {
      return;
    }
    const isEdit = this.text === 'Add' ? false : true;
    this.isLoading = true;
    if (isEdit) {
      const accountData = { ...this.editingAccount, ...form.value } as Account;
      await this.dbService.editAccount(accountData);
    } else {
      const accountData = form.value as Account;
      (accountData.budgetId = this.store.selectedBudet), (accountData.closed = false);
      accountData.deleted = false;
      const startingBalPayee = this.store.payees$.value.find((payee) => payee.name === STARTING_BALANCE_PAYEE)!;
      await this.dbService.createAccount(accountData, this.store.inflowCategory$.value!, startingBalPayee);
    }
    this.isLoading = false;
    this.resetAccountForm();
  }

  selectComponent(component: SelectedComponent) {
    this.store.selectedAccount = null;
    this.store.selectedComponent = component;
  }

  async addBudget() {
    // if empty return
    if (!this.newBudgetName) {
      return;
    }
    const budget: Budget = {
      name: this.newBudgetName,
      isSelected: false,
    };
    await this.dbService.createBudget(budget);
  }

  async selectBudget(budget: Budget) {
    budget.isSelected = true;
    const selectedBudget = this.store.selectedBudget$.value;
    if (selectedBudget) {
      selectedBudget.isSelected = false;
      // update selected to unselected
      await this.dbService.editBudget(selectedBudget);
    }
    // update to selected
    await this.dbService.editBudget(budget);
  }

  selectAccount(account: Account) {
    this.store.selectedAccount = account;
    this.store.selectedComponent = SelectedComponent.ACCOUNTS;
  }

  async closeAccount() {
    const acc = { ...this.editingAccount, closed: true };
    await this.dbService.editAccount(acc);
  }
}
