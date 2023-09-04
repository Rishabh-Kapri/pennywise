import { AfterViewInit, Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { DatabaseService } from '../services/database.service';
import { Account, AccountType, AccountTypeNames } from '../models/account.model';
import { FormControl, FormGroup, Validators } from '@angular/forms';
import { StoreService } from '../services/store.service';
import { catchError, map, startWith, tap } from 'rxjs/operators';
import { Observable, of } from 'rxjs';
import { Modal, ModalOptions } from 'flowbite';
import { SelectedComponent } from '../models/state.model';
import { Budget } from '../models/budget.model';

interface AccountForm {
  name: FormControl<string | null>;
  type: FormControl<AccountType | null>;
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
  selectedComponent = SelectedComponent;
  text = 'Add';
  addAcountModal: Modal;
  accountTypeNames = AccountTypeNames;
  accountForm: FormGroup<AccountForm>;

  selectedAccount: Account;
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
    this.totalCurrentFunds$ = this.store.accounts$?.pipe(
      map((data) => data.reduce((a, b) => a + b.balance, 0)),
      tap(console.log)
    );
    this.unSelectedBudgets$ = this.store.budget$.pipe(
      map((budgets) => budgets.filter((budget) => budget.isSelected === false))
    );
    this.store.accounts$?.pipe(
      map((data) => ({ isLoading: true, data })),
      catchError((error) => of({ isLoading: false, error })),
      startWith({ isLoading: true })
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
      accountData.budgetId = this.store.selectedBudget$.value?.id!;
      accountData.closed = false;
      accountData.deleted = false;
      accountData.createdAt = new Date().toISOString();
      await this.dbService.createAccount(accountData);
    }
    this.isLoading = false;
    this.resetAccountForm();
  }

  selectComponent(component: SelectedComponent) {
    this.store.selectedComponent = component;
  }

  async addBudget() {
    // if empty return
    if (!this.newBudgetName) {
      return;
    }
    const budget: Budget = {
      name: this.newBudgetName,
      createdAt: new Date().toISOString(),
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
}
