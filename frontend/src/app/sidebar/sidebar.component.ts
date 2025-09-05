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
import { Store } from '@ngxs/store';
import { CategoryGroupsState } from '../store/dashboard/states/categoryGroups/categoryGroups.state';
import { BudgetsState } from '../store/dashboard/states/budget/budget.state';
import { AccountsState } from '../store/dashboard/states/accounts/accounts.state';
import { PayeesState } from '../store/dashboard/states/payees/payees.state';
import { CategoriesState } from '../store/dashboard/states/categories/categories.state';
import { AccountsActions } from '../store/dashboard/states/accounts/accounts.action';
import { BudgetsActions } from '../store/dashboard/states/budget/budget.action';
import { ConfigActions } from '../store/dashboard/states/config/config.action';
import { ConfigState } from '../store/dashboard/states/config/config.state';

interface AccountForm {
  name: FormControl<string | null>;
  type: FormControl<BudgetAccountType | TrackingAccountType | null>;
  balance: FormControl<number | null>;
}

@Component({
  selector: 'app-sidebar',
  templateUrl: './sidebar.component.html',
  styleUrls: ['./sidebar.component.scss'],
  standalone: false,
})
export class SidebarComponent implements OnInit, AfterViewInit {
  @ViewChild('addAccountModal', { read: ElementRef }) addAccountModalRef: ElementRef<HTMLElement>;
  modalOptions: ModalOptions = {
    backdrop: 'dynamic',
    backdropClasses: 'bg-gray-900 bg-opacity-50 dark:bg-opacity-80 fixed inset-0 z-40',
    closable: true,
  };
  selectedComponent = SelectedComponent;
  text = 'Add';
  addAcountModal: Modal;
  accountForm: FormGroup<AccountForm>;
  budgetAccountNames = BudgetAccountNames;
  trackingAccountNames = TrackingAccountNames;

  editingAccount: Account;

  isLoading = false;
  newBudgetName = '';
  
  // Mobile sidebar state
  isMobileSidebarOpen = false;

  budgetAccountData$: Observable<{ totalAmount: number; accounts: Account[] }>;
  trackingAccountData$: Observable<{ totalAmount: number; accounts: Account[] }>;

  totalCurrentFunds$ = this.ngxsStore.select(AccountsState.getTotalCurrentFunds);
  allBudgets$ = this.ngxsStore.select(BudgetsState.getAllBudgets);
  selectedBudget$ = this.ngxsStore.select(BudgetsState.getSelectedBudget);
  unSelectedBudgets$ = this.ngxsStore.select(BudgetsState.getUnselectedBudgets);
  allAccounts$ = this.ngxsStore.select(AccountsState.getAllAccounts);
  selectedAccount$ = this.ngxsStore.select(AccountsState.getSelectedAccount);
  closedAccounts$ = this.ngxsStore.select(AccountsState.getClosedAccounts);
  budgetAccounts$ = this.ngxsStore.select(AccountsState.getBudgetAccounts);
  trackingAccounts$ = this.ngxsStore.select(AccountsState.getTrackingAccounts);
  inflowWithBalance$ = this.ngxsStore.select(CategoriesState.getInflowWithBalance);
  selectedComponent$ = this.ngxsStore.select(ConfigState.getSelectedComponent);

  constructor(
    private dbService: DatabaseService,
    private ngxsStore: Store,
    public store: StoreService,
  ) {}

  ngOnInit(): void {
    this.accountForm = new FormGroup<AccountForm>({
      name: new FormControl('', { validators: [Validators.required], nonNullable: true }),
      type: new FormControl(null, { validators: [Validators.required] }),
      balance: new FormControl(null, { validators: [Validators.required] }),
    });
    this.budgetAccountData$ = combineLatest([this.budgetAccounts$]).pipe(
      switchMap(([accounts]) => {
        let data = {
          totalAmount: Number(accounts.reduce((a, b) => a + (b.balance ?? 0), 0).toFixed(2)),
          accounts,
        };
        return of(data);
      }),
    );
    this.trackingAccountData$ = combineLatest([this.trackingAccounts$]).pipe(
      switchMap(([accounts]) => {
        let data = {
          totalAmount: Number(accounts.reduce((a, b) => a + (b.balance ?? 0), 0).toFixed(2)),
          accounts,
        };
        return of(data);
      }),
    );
  }

  ngAfterViewInit(): void {
    this.addAcountModal = new Modal(this.addAccountModalRef.nativeElement, this.modalOptions);
  }

  // Mobile sidebar methods
  public toggleMobileSidebar() {
    this.isMobileSidebarOpen = !this.isMobileSidebarOpen;
  }

  public closeMobileSidebar() {
    this.isMobileSidebarOpen = false;
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
      accountData.budgetId = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget)?.id ?? '';
      accountData.closed = false;
      accountData.deleted = false;
      const startingBalPayee = this.ngxsStore.selectSnapshot(PayeesState.getStartingBalancePayee)!;
      await this.dbService.createAccount(
        accountData,
        this.ngxsStore.selectSnapshot(CategoriesState.getInflowWithBalance)!,
        startingBalPayee,
      );
    }
    this.isLoading = false;
    this.resetAccountForm();
  }

  selectComponent(component: SelectedComponent) {
    this.ngxsStore.dispatch(new AccountsActions.SetSelectedAccount(null));
    this.ngxsStore.dispatch(new ConfigActions.SetSelectedComponent(component));
    // Close mobile sidebar when navigating
    this.closeMobileSidebar();
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
    const selectedBudget = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget);
    if (selectedBudget) {
      selectedBudget.isSelected = false;
      // update selected to unselected
      await this.dbService.editBudget(selectedBudget);
    }
    // update to selected
    await this.dbService.editBudget(budget);
    this.ngxsStore.dispatch(new BudgetsActions.SetSelectedBudget(budget));
  }

  selectAccount(account: Account) {
    this.ngxsStore.dispatch(new AccountsActions.SetSelectedAccount(account));
    this.ngxsStore.dispatch(new ConfigActions.SetSelectedComponent(SelectedComponent.ACCOUNTS));
    // Close mobile sidebar when selecting account
    this.closeMobileSidebar();
  }

  async closeAccount() {
    const acc = { ...this.editingAccount, closed: true };
    await this.dbService.editAccount(acc);
  }
}
