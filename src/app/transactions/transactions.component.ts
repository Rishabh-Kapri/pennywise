import {
  ChangeDetectorRef,
  Component,
  Input,
  OnChanges,
  OnDestroy,
  SimpleChanges,
  TemplateRef,
  ViewContainerRef,
} from '@angular/core';
import { Account, TrackingAccountType } from '../models/account.model';
import { StoreService } from '../services/store.service';
import { map, switchMap } from 'rxjs/operators';
import { BehaviorSubject, Observable, combineLatest, of } from 'rxjs';
import { Category } from '../models/category.model';
import {
  AllAccountsColumns,
  NormalizedTransaction,
  SelectedAccountColumns,
  Transaction,
} from '../models/transaction.model';
import { HelperService } from '../services/helper.service';
import { CategoryGroupData } from '../models/state.model';
import { Payee } from '../models/payee.model';
import { DatabaseService } from '../services/database.service';
import { PopoverService } from '../services/popover.service';
import { PopoverRef } from '../services/popover-ref';
import { Parser } from 'expr-eval';
import { INFLOW_CATEGORY_NAME, STARTING_BALANCE_PAYEE } from '../constants/general';

declare var Datepicker: any;

enum Mode {
  NONE, // no editing/creation is ongoing
  SELECT, // a transaction is currently selected
  CREATE,
  EDIT,
}
interface PayeesData {
  Transfers: Payee[];
  Saved: Payee[];
}

@Component({
  selector: 'app-transactions',
  templateUrl: './transactions.component.html',
  styleUrls: ['./transactions.component.scss'],
})
export class TransactionsComponent implements OnChanges, OnDestroy {
  @Input() account: Account | null;
  parser = new Parser();
  mode = Mode;
  totalCurrentFunds$: Observable<number>;
  categoryObj$: Observable<Record<string, Category>>;
  trackingAccountType = TrackingAccountType;

  transactionColumns: Array<{ name: string; class: string }> = [];
  transactionColumnsObj: Record<string, { name: string; class: string }>;
  selectedTransaction: NormalizedTransaction | null;
  selectedAccount: Account;
  selectedPayee: Payee;
  currentMode: Mode = Mode.NONE;

  accountData$: Observable<{ name: string; accounts: Account[] }[]>;
  categoryGroupData$: Observable<CategoryGroupData[]>;
  payeesData$: Observable<PayeesData>;
  searchCategory$ = new BehaviorSubject<string>('');
  searchPayee$ = new BehaviorSubject<string>('');
  searchAccount$ = new BehaviorSubject<string>('');

  payeeOverlayRef: PopoverRef;
  categoryOverlayRef: PopoverRef;
  accountOverlayRef: PopoverRef;

  constructor(
    public store: StoreService,
    public helperService: HelperService,
    private cdRef: ChangeDetectorRef,
    private dbService: DatabaseService,
    private viewContainerRef: ViewContainerRef,
    private popper: PopoverService
  ) {
    this.accountData$ = combineLatest([this.store.budgetAccounts$, this.store.trackingAccounts$]).pipe(
      switchMap(([budgetAccounts, trackingAccounts]) => {
        const groupData = [
          { name: 'Budget Accounts', accounts: budgetAccounts },
          { name: 'Tracking Accounts', accounts: trackingAccounts },
        ];
        return of(groupData);
      })
    );
    this.categoryGroupData$ = combineLatest([
      this.store.categoryGroupData$,
      this.store.inflowCategory$,
      this.searchCategory$,
    ]).pipe(
      switchMap(([categoryGroupData, inflowCategory, search]) => {
        const selectedMonth = this.store.selectedMonth;
        const inflowGroup: CategoryGroupData = {
          name: 'Inflow',
          id: '1',
          balance: 0,
          budgeted: 0,
          activity: 0,
          categories: [
            {
              id: inflowCategory?.id!,
              name: inflowCategory?.name!,
              budgetId: inflowCategory?.budgetId!,
              budgeted: { [selectedMonth]: 0 },
              activity: { [selectedMonth]: 0 },
              balance: { [selectedMonth]: inflowCategory?.budgeted ?? 0 },
              categoryGroupId: '1',
            },
          ],
        };
        const searchStr = search.toLowerCase();
        const data = categoryGroupData.filter((group) => {
          return (
            group.name.toLowerCase().includes(searchStr) ||
            group.categories.filter((category) => {
              const filtered = category.name.toLowerCase().includes(searchStr);
              return filtered;
            }).length
          );
        });
        return of([inflowGroup, ...data]);
      })
    );

    this.payeesData$ = combineLatest([this.store.payees$, this.searchPayee$]).pipe(
      switchMap(([payees, search]) => {
        const payeesData: PayeesData = {
          Transfers: [],
          Saved: [],
        };
        const searchStr = search.toLowerCase();
        for (let payee of payees) {
          if (payee.id !== this.account?.transferPayeeId && payee.name !== STARTING_BALANCE_PAYEE) {
            if (payee.name.toLowerCase().includes(searchStr)) {
              if (payee.transferAccountId) {
                payeesData.Transfers.push(payee);
              } else {
                payeesData.Saved.push(payee);
              }
            }
          }
        }
        return of(payeesData);
      })
    );
  }

  ngOnInit(): void {
    this.categoryObj$ = this.store.categories$.pipe(
      map((categories) => {
        return categories.reduce((obj: Record<string, Category>, category: Category) => {
          const data = Object.assign(obj, { [category.id!]: category });
          return data;
        }, {});
      })
    );
  }

  ngOnChanges(changes: SimpleChanges): void {
    this.cancelTransactionSave();
    if (changes['account']) {
      this.setAccountData();
      this.selectedAccount = changes['account'].currentValue;
    }
  }

  ngOnDestroy(): void {
    this.cancelTransactionSave();
  }

  searchAccount(event: any) {
    this.searchAccount$.next(event.target.value);
    this.cdRef.detectChanges();
  }

  searchCategory(event: any) {
    this.searchCategory$.next(event.target.value);
    this.cdRef.detectChanges();
  }

  searchPayee(event: any) {
    this.searchPayee$.next(event.target.value);
    this.cdRef.detectChanges();
  }

  setAccountData() {
    this.transactionColumns = [];
    if (this.account) {
      this.transactionColumns = structuredClone(SelectedAccountColumns);
      this.totalCurrentFunds$ = of(this.account.balance);
    } else {
      this.transactionColumns = structuredClone(AllAccountsColumns);
      this.totalCurrentFunds$ = this.store.accounts$?.pipe(map((data) => data.reduce((a, b) => a + b.balance, 0)));
    }
    this.transactionColumnsObj = structuredClone(this.transactionColumns).reduce((obj, col) => {
      return Object.assign(obj, { [col.name]: col });
    }, {});
  }

  addTransaction() {
    this.currentMode = Mode.CREATE;
    this.selectedTransaction = {
      transferTransactionId: null,
      accountName: '',
      accountId: '',
      budgetId: this.store.selectedBudet,
      date: '',
      outflow: null,
      inflow: null,
      balance: 0,
      payeeName: '',
      payeeId: '',
      categoryName: '',
      categoryId: '',
    };
    this.searchAccount$.next('');
    this.searchPayee$.next('');
    this.searchCategory$.next('');
  }

  selectTransaction(transaction: NormalizedTransaction) {
    if (this.selectedTransaction?.id === transaction.id) {
      return;
    }
    // @TODO: for now setting mode as edit on select, change later
    this.currentMode = Mode.EDIT;
    const account = this.store.accounts$.value.find((acc) => acc.id! === transaction.accountId);
    if (account) {
      this.selectedAccount = account;
    }
    const payee = this.store.payees$.value.find((payee) => payee.id! === transaction.payeeId);
    if (payee) {
      this.selectedPayee = payee;
    }
    this.selectedTransaction = structuredClone(transaction);
    this.searchAccount$.next(transaction.accountName);
    this.searchPayee$.next(transaction.payeeName);
    this.searchCategory$.next(transaction.categoryName ?? 'Category Not Needed');
  }

  openDatePicker(transaction: Transaction) {
    const datePickerEl = document.getElementById(`datePicker-${transaction.id}`);
    new Datepicker(datePickerEl, {});
  }

  changeTransactionDate(date: string, transaction: NormalizedTransaction) {
    console.log(date, transaction);
  }

  showAccountSelectMenu(content: TemplateRef<any>, origin: HTMLInputElement) {
    if (this.accountOverlayRef?.isOpen) {
      return;
    }
    this.accountOverlayRef = this.popper.open({ origin, content, viewContainerRef: this.viewContainerRef });
  }

  closeAccountSelectMenu() {
    if (this.accountOverlayRef?.isOpen) {
      this.accountOverlayRef.close();
    }
  }

  showPayeeSelectMenu(content: TemplateRef<any>, origin: HTMLInputElement) {
    if (this.payeeOverlayRef?.isOpen) {
      return;
    }
    this.payeeOverlayRef = this.popper.open({ origin, content, viewContainerRef: this.viewContainerRef });
  }

  closePayeeSelectMenu() {
    if (this.payeeOverlayRef?.isOpen) {
      this.payeeOverlayRef.close();
    }
  }

  showCategorySelectMenu(content: TemplateRef<any>, origin: HTMLInputElement) {
    if (this.categoryOverlayRef?.isOpen) {
      return;
    }
    this.categoryOverlayRef = this.popper.open({ origin, content, viewContainerRef: this.viewContainerRef });
  }

  closeCategorySelectMenu() {
    if (this.categoryOverlayRef?.isOpen) {
      this.categoryOverlayRef.close();
    }
  }

  changeAmount(field: 'inflow' | 'outflow') {
    if (this.selectedTransaction) {
      // if inflow is selected reset outflow, and vice versa
      if (field === 'inflow') {
        this.selectedTransaction.outflow = null;
      } else if (field === 'outflow') {
        this.selectedTransaction.inflow = null;
      }
    }
  }

  setAmount(field: 'inflow' | 'outflow', event: any) {
    // normalize calculations
    try {
      const expr = this.parser.parse(event.target.value);
      if (this.selectedTransaction) {
        this.selectedTransaction[field] = expr.evaluate();
      }
    } catch (err) { }
  }

  async createNewPayee() {
    const payee: Payee = {
      name: this.searchPayee$.value,
      budgetId: this.store.selectedBudet,
      transferAccountId: null,
      deleted: false,
      createdAt: new Date().toISOString(),
    };
    return await this.dbService.createPayee(payee);
  }

  selectAccount(account: Account) {
    this.selectedAccount = account;
    if (this.selectedTransaction) {
      this.selectedTransaction.accountId = account.id!;
      this.selectedTransaction.accountName = account.name;
    }
    this.searchAccount$.next(account.name);
    this.closeAccountSelectMenu();
  }

  async selectPayee(event: 'enter' | 'click', payee?: Payee) {
    if (payee) {
      this.selectedPayee = payee;
    }
    let selectedPayeeId = payee?.id!;
    let selectedPayeeName = payee?.name!;
    if (event === 'enter') {
      const allPayees = this.store.payees$.value;
      const searchPayee = this.searchPayee$.value;
      const filteredPayee = allPayees.find((payee) => payee.name.toLowerCase() === searchPayee.toLowerCase());
      if (filteredPayee) {
        // if payee exists then select it, otherwise create it
        selectedPayeeId = filteredPayee.id!;
        selectedPayeeName = filteredPayee.name;
      } else {
        const createdPayee = await this.createNewPayee();
        // then select this new created payee
        selectedPayeeId = createdPayee.id;
        selectedPayeeName = searchPayee;
      }
    }
    if (this.selectedTransaction) {
      const budgetAccounts = this.store.budgetAccounts$.value;
      const isSelectedAccBudget = budgetAccounts.find((acc) => acc.id! === this.selectedPayee?.transferAccountId!);
      console.log(isSelectedAccBudget, this.selectedPayee, budgetAccounts);
      if (isSelectedAccBudget) {
        this.selectedTransaction.categoryId = null;
        this.selectedTransaction.categoryName = null;
        this.searchCategory$.next('Category Not Needed');
      } else {
        this.selectedTransaction.categoryId = '';
        this.selectedTransaction.categoryName = '';
        this.searchCategory$.next('');
      }
      this.selectedTransaction.payeeId = selectedPayeeId;
      this.selectedTransaction.payeeName = selectedPayeeName;
    }
    this.searchPayee$.next(selectedPayeeName);
    this.closePayeeSelectMenu();
  }

  selectCategory(category: Category) {
    console.log('selecting category:', category);
    if (this.selectedTransaction) {
      this.selectedTransaction.categoryId = category.id!;
      this.selectedTransaction.categoryName = category.name;
    }
    this.searchCategory$.next(category.name);
    this.closeCategorySelectMenu();
  }

  deleteTransaction(transaction: NormalizedTransaction) { }

  cancelTransactionSave() {
    this.currentMode = Mode.NONE;
    this.selectedTransaction = null;
  }

  async saveTransaction() {
    if (this.selectedTransaction) {
      if (this.selectedAccount && !this.selectedTransaction.accountId) {
        this.selectedTransaction.accountId = this.selectedAccount.id!;
        this.selectedTransaction.accountName = this.selectedAccount.name;
      }
      const amount = this.selectedTransaction.inflow ?? -this.selectedTransaction?.outflow! ?? 0;
      switch (this.currentMode) {
        case Mode.CREATE:
          if (this.selectedTransaction.categoryName === INFLOW_CATEGORY_NAME) {
            // update inflow category
            const inflowCategory = this.store.inflowCategory$.value;
            if (inflowCategory) {
              this.dbService.editCategory({
                id: inflowCategory.id!,
                budgeted: inflowCategory.budgeted + amount,
              });
            }
          }
          this.createNewTransaction(this.selectedTransaction, amount, this.selectedAccount, this.selectedPayee);
          this.cancelTransactionSave();
          break;
        case Mode.EDIT:
          let existingTransaction = this.store.transactions$.value.find(
            (tran) => tran.id === this.selectedTransaction?.id
          );
          const inflowCategory = this.store.inflowCategory$.value;
          // when payee is changed
          const isNewTransaction =
            this.selectedTransaction.payeeId !== existingTransaction?.payeeId ||
            this.selectedTransaction.accountId !== existingTransaction?.accountId;

          // update amount in inflow category
          if (
            this.selectedTransaction.categoryId === inflowCategory?.id &&
            existingTransaction?.categoryId !== inflowCategory?.id
          ) {
            // if new category is inflow and older wasn't
            // add amount to inflow
            this.dbService.editCategory({
              id: inflowCategory.id!,
              budgeted: inflowCategory.budgeted + amount,
            });
          } else if (
            existingTransaction?.categoryId === inflowCategory?.id &&
            this.selectedTransaction.categoryId !== inflowCategory?.id
          ) {
            // if new category isn't inflow and older was
            // subtract amount from inflow
            this.dbService.editCategory({
              id: inflowCategory?.id,
              budgeted: (inflowCategory?.budgeted ?? 0) - (existingTransaction?.amount ?? 0),
            });
          } else if (
            existingTransaction?.categoryId === inflowCategory?.id &&
            this.selectedTransaction.categoryId === inflowCategory?.id
          ) {
            // if old and new categories are inflow
            const diff = amount - (existingTransaction?.amount ?? 0);
            this.dbService.editCategory({
              id: inflowCategory.id,
              budgeted: inflowCategory.budgeted + diff,
            });
          }

          if (this.selectedPayee.transferAccountId) {
            if (existingTransaction?.transferTransactionId) {
              if (isNewTransaction) {
                // changing payees and accounts
                // this is a completely new transaction
                console.log('new transaction');
                // delete both existingTransaction and existing transferTransaction
                this.dbService.deleteTransaction(existingTransaction.id!);
                this.dbService.deleteTransaction(existingTransaction.transferTransactionId);
                this.createNewTransaction(this.selectedTransaction, amount, this.selectedAccount, this.selectedPayee);
              } else if (
                this.selectedTransaction.payeeId !== existingTransaction.payeeId &&
                this.selectedTransaction.accountId === existingTransaction.accountId
              ) {
                console.log('changed transfer payees');
                // only changing payees
                // @TODO: subtract amount from existing transfer payee account
                // @TODO: add amount to selected transfer payee account
                // @TODO: update tranferTransaction: accountId, amount
                // @TODO: update selectTransaction: payeeId, amount
              } else {
                // only changing accounts
                // update selected transaction
                console.log('Selected:', this.selectedTransaction, 'Existing:', existingTransaction);
                this.dbService.editTransaction({
                  id: this.selectedTransaction.id!,
                  amount,
                  categoryId: this.selectedTransaction.categoryId,
                  note: this.selectedTransaction.note,
                  date: this.selectedTransaction.date,
                });
                // update transfer transaction
                this.dbService.editTransaction({
                  id: existingTransaction.transferTransactionId!,
                  amount: -amount,
                  categoryId: this.selectedTransaction.categoryId,
                  note: this.selectedTransaction.note,
                  date: this.selectedTransaction.date,
                });
              }
            } else {
              console.log('creating new transfer transaction');
              // this means the original was normal payee, and new is transfer
              // in this case the payees cannot be similar, only accounts can be
              this.handleTransferTransaction(
                -amount,
                this.selectedTransaction.accountId,
                this.selectedTransaction.id!,
                this.selectedPayee.id!,
                this.selectedPayee.transferAccountId,
                this.selectedAccount.transferPayeeId!
              );
            }
          } else {
            // normal payee is selected
            if (existingTransaction?.transferTransactionId) {
              console.log('deleting transfer transaction');
              // previous payee was transfer payee, delete the transfer transaction
              await this.dbService.editTransaction({
                id: existingTransaction.transferTransactionId,
                deleted: true,
              });
              await this.dbService.editTransaction({
                id: this.selectedTransaction.id!,
                payeeId: this.selectedTransaction.payeeId,
                amount,
                categoryId: this.selectedTransaction.categoryId,
                note: this.selectedTransaction.note,
                date: this.selectedTransaction.date,
                transferTransactionId: null,
                transferAccountId: null,
              });
            } else {
              // previous payee was normal, just update the payeeId
              console.log('current & previous normal payees');
              await this.dbService.editTransaction({
                id: this.selectedTransaction.id,
                payeeId: this.selectedTransaction.payeeId,
                amount,
                categoryId: this.selectedTransaction.categoryId,
                note: this.selectedTransaction.note,
                date: this.selectedTransaction.date,
              });
            }
          }
          this.cancelTransactionSave();
          break;
      }
    }
  }

  async createNewTransaction(selectedTransaction: NormalizedTransaction, amount: number, selectedAccount: Account, selectedPayee: Payee) {
    const newTransaction: Transaction = {
      budgetId: this.store.selectedBudet,
      date: selectedTransaction.date,
      amount: amount,
      accountId: selectedTransaction.accountId,
      payeeId: selectedTransaction.payeeId,
      categoryId: selectedTransaction.categoryId,
      note: selectedTransaction.note ?? '',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      deleted: false,
    };
    const createdTransac = await this.dbService.createTransaction(newTransaction);
    if (selectedPayee?.transferAccountId) {
      // this is a transfer payee, create another transaction
      this.handleTransferTransaction(
        -amount,
        newTransaction.accountId,
        createdTransac.id,
        this.selectedPayee.id!,
        selectedPayee.transferAccountId!,
        selectedAccount?.transferPayeeId!
      );
    }
  }

  /**
   * @description
   * - Handles creation of transfer transaction, the transaction that is associated with transfer payee
   * - It also edits the current transaction with the updated data
   * @param {number} amount the amount for this transaction (make sure amount is negative for outflow)
   * @param {string} accountId the accountId of the current transaction
   * @param {string} transactionId the transaction id which this transfer transaction is associated with
   * @param {string} payeeId the payee id which the current transaction has been selected
   * @param {string} transferAccountId the id of the account this transaction is of, this will be the account the transfer is being made to
   * @param {string} transferPayeeId the id of the payee
   */
  async handleTransferTransaction(
    amount: number,
    accountId: string,
    transactionId: string,
    payeeId: string,
    transferAccountId: string,
    transferPayeeId: string
  ) {
    const transferTransac: Transaction = {
      budgetId: this.store.selectedBudet,
      transferTransactionId: transactionId,
      transferAccountId: accountId,
      date: this.selectedTransaction?.date!,
      amount: amount,
      accountId: transferAccountId,
      payeeId: transferPayeeId!,
      // @NOTE: for now tranfer transaction won't have a category since I can only transfer between budget accounts or from budget to tracking accounts
      // transactions cannot be created from tracking accounts, only to them from budget accounts, either inflow or outflow
      categoryId: null, 
      note: this.selectedTransaction?.note ?? '',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      deleted: false,
    };
    console.log('Transfer Transaction:', transferTransac, 'Selected Transaction:', this.selectedTransaction);
    const transferCreatedTransac = await this.dbService.createTransaction(transferTransac);
    await this.dbService.editTransaction({
      id: transactionId,
      payeeId,
      amount: -amount,
      transferTransactionId: transferCreatedTransac.id,
      transferAccountId,
      categoryId: this.selectedTransaction?.categoryId,
      note: this.selectedTransaction?.note,
      date: this.selectedTransaction?.date,
    });
  }
}
