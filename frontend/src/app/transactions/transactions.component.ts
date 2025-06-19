import {
  ChangeDetectionStrategy,
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
import { debounceTime, filter, map, startWith, switchMap } from 'rxjs/operators';
import { BehaviorSubject, Observable, combineLatest, of } from 'rxjs';
import { Category } from '../models/category.model';
import {
  AllAccountsColumns,
  NormalizedTransaction,
  SearchColumns,
  SelectedAccountColumns,
  Transaction,
  TransactionSearchKeys,
} from '../models/transaction.model';
import { HelperService } from '../services/helper.service';
import { CategoryGroupData } from '../models/state.model';
import { Payee } from '../models/payee.model';
import { DatabaseService } from '../services/database.service';
import { PopoverService } from '../services/popover.service';
import { PopoverRef } from '../services/popover-ref';
import { Parser } from 'expr-eval';
import { STARTING_BALANCE_PAYEE } from '../constants/general';
import { Store } from '@ngxs/store';
import { TransactionsState } from '../store/dashboard/states/transactions/transactions.state';
import { AccountsState } from '../store/dashboard/states/accounts/accounts.state';
import { CategoryGroupsState } from '../store/dashboard/states/categoryGroups/categoryGroups.state';
import { CategoriesState } from '../store/dashboard/states/categories/categories.state';
import { PayeesState } from '../store/dashboard/states/payees/payees.state';
import { BudgetsState } from '../store/dashboard/states/budget/budget.state';
import { PayeesActions } from '../store/dashboard/states/payees/payees.action';

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
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
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
  filteredTransactions$: Observable<NormalizedTransaction[]>;
  payeesData$: Observable<PayeesData>;
  searchTransations$ = new BehaviorSubject<string>('');
  searchCategory$ = new BehaviorSubject<string>('');
  searchPayee$ = new BehaviorSubject<string>('');
  searchAccount$ = new BehaviorSubject<string>('');

  allTransactions$ = this.ngxsStore.select(TransactionsState.getAllTransactions);
  normalizedTransactions$ = this.ngxsStore.select(TransactionsState.getNormalizedTransaction);
  budgetAccounts$ = this.ngxsStore.select(AccountsState.getBudgetAccounts);
  trackingAccounts$ = this.ngxsStore.select(AccountsState.getTrackingAccounts);

  categoryGroupsData$ = this.ngxsStore.select(CategoryGroupsState.getCategoryGroupData);
  inflowCategory$ = this.ngxsStore.select(CategoriesState.getInflowWithBalance);
  allPayees$ = this.ngxsStore.select(PayeesState.getAllPayees);
  allCategories$ = this.ngxsStore.select(CategoriesState.getAllCategories);
  allAccounts$ = this.ngxsStore.select(AccountsState.getAllAccounts);
  selectedMonth$ = this.ngxsStore.select(BudgetsState.getSelectedMonth);
  // totalCurrentFunds$ = this.ngxsStore.select(AccountsState.getTotalCurrentFunds);

  payeeOverlayRef: PopoverRef;
  categoryOverlayRef: PopoverRef;
  accountOverlayRef: PopoverRef;

  searching = false;

  constructor(
    public store: StoreService,
    public helperService: HelperService,
    private ngxsStore: Store,
    private cdRef: ChangeDetectorRef,
    private dbService: DatabaseService,
    private viewContainerRef: ViewContainerRef,
    private popper: PopoverService,
  ) {
    this.accountData$ = combineLatest([this.budgetAccounts$, this.trackingAccounts$]).pipe(
      switchMap(([budgetAccounts, trackingAccounts]) => {
        const groupData = [
          { name: 'Budget Accounts', accounts: budgetAccounts },
          { name: 'Tracking Accounts', accounts: trackingAccounts },
        ];
        return of(groupData);
      }),
    );

    this.filteredTransactions$ = combineLatest([
      this.normalizedTransactions$,
      this.searchTransations$.pipe(startWith('')),
    ]).pipe(
      debounceTime(750),
      filter(() => !this.searching),
      switchMap(([normalizedTransactions, searchTransations]) => {
        this.searching = true;
        const search = searchTransations.toLowerCase();
        const multiSearchArr = search.split(',');

        let filters: any = {};
        let searchTerm: string = '';

        // multiSearchArr.forEach(([key, value]) => {
        //   console.log(key, value);
        //   if (value) {
        //     filters[key] = value.trim();
        //   } else {
        //     searchTerm = value.trim();
        //   }
        // })
        searchTerm = '';
        console.log(multiSearchArr);
        for (const arr of multiSearchArr) {
          const searchArr = arr.split(':');
          if (searchArr.length > 1) {
            filters[searchArr[0]] = searchArr[1];
            // col = [...col, searchArr[0]];
            searchTerm = searchArr[1];
          } else {
            searchTerm = searchArr[0];
          }
        }
        for (const filterKey of Object.keys(filters)) {
          if (filterKey === 'payee') {
            filters.payee = filters.payee.split('|');
          } else if (filterKey === 'category') {
            filters.category = filters.category.split('|');
          } else if (filterKey === 'date') {
            const dateArr: string[] = filters.date.split('-');
            if (filters.date.length) {
              filters.date = {
                from: '',
                to: '',
              };
            }
            const [from, to] = dateArr;
            if (from) {
              const [fromYear, fromMonth, fromDay] = from.split('/');
              const parsedFromMonth = fromMonth ?? '01';
              const parsedFromDay = fromDay ?? '01';
              const parsedFromDate = `${fromYear}-${parsedFromMonth}-${parsedFromDay}`;
              filters.date.from = parsedFromDate;
              if (to) {
                const [toYear, toMonth, toDay] = to.split('/');
                const parsedToDate = `${toYear}-${toMonth ?? '01'}-${toDay ?? '01'}`;
                filters.date.to = parsedToDate;
              } else {
                if (fromYear) {
                  if (!fromMonth) {
                    // if only year is present set to date to end of year
                    filters.date.to = `${Number(fromYear) + 1}-${parsedFromMonth}-${parsedFromDay}`;
                  } else if (fromMonth && !fromDay) {
                    // if only year and month is present
                    const toDate = new Date(Number(fromYear), Number(fromMonth), Number(parsedFromDay));
                    filters.date.to = `${toDate.getFullYear()}-${toDate.getMonth() + 1}-${toDate.getDate()}`;
                  } else if (fromMonth && fromDay) {
                    // if all year, month and day is present
                    const toDate = new Date(Number(fromYear), Number(fromMonth) - 1, Number(fromDay) + 1);
                    filters.date.to = `${toDate.getFullYear()}-${toDate.getMonth() + 1}-${toDate.getDate()}`;
                  }
                }
              }
            }
          }
        }
        console.log(filters);
        let tempTxns = normalizedTransactions;
        if (searchTerm) {
          if (Object.keys(filters).length) {
            for (const key of Object.keys(filters)) {
              const k = SearchColumns[key as keyof typeof SearchColumns] as TransactionSearchKeys;
              if (key === 'date') {
                tempTxns = tempTxns.filter((txn) => {
                  return this.filterByDate(filters[key], txn);
                });
              } else if (key === 'payee' || key === 'category') {
                tempTxns = tempTxns.filter((txn) => {
                  return filters[key].some((payee: string) => txn[k]?.toLowerCase().includes(payee));
                });
              } else {
                tempTxns = tempTxns.filter((txn) => {
                  return txn[k]?.toLowerCase().includes(filters[key]);
                });
              }
            }
          } else {
            tempTxns = tempTxns.filter((txn) => {
              return (
                txn.accountName.toLowerCase().includes(searchTerm) ||
                txn.payeeName.toLowerCase().includes(searchTerm) ||
                txn.categoryName?.toLowerCase().includes(searchTerm) ||
                txn.note?.toLowerCase().includes(searchTerm) ||
                txn.date?.includes(searchTerm)
              );
            });
          }
        }
        console.log(
          'COST:',
          tempTxns.reduce((val, txn) => -(txn.outflow ?? 0) + (txn.inflow ?? 0) + val, 0),
        );
        this.searching = false;
        return of(tempTxns);
      }),
    );

    this.categoryGroupData$ = combineLatest([
      this.categoryGroupsData$,
      this.inflowCategory$,
      this.searchCategory$,
    ]).pipe(
      switchMap(([categoryGroupData, inflowCategory, search]) => {
        const selectedMonth = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth);
        const inflowGroup: CategoryGroupData = {
          name: 'Inflow',
          id: '1',
          balance: {
            [selectedMonth]: 0,
          },
          budgeted: {
            [selectedMonth]: 0,
          },
          activity: {
            [selectedMonth]: 0,
          },
          collapsed: false,
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
      }),
    );

    this.payeesData$ = combineLatest([this.allPayees$, this.searchPayee$]).pipe(
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
      }),
    );
  }

  ngOnInit(): void {
    this.categoryObj$ = this.allCategories$.pipe(
      map((categories) => {
        return categories.reduce((obj: Record<string, Category>, category: Category) => {
          const data = Object.assign(obj, { [category.id!]: category });
          return data;
        }, {});
      }),
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

  private filterByDate(dateObj: { from: string; to: string }, transaction: NormalizedTransaction): boolean {
    let returnValue = false;
    const txnDate = new Date(transaction.date);
    const fromDate = new Date(dateObj.from);
    if (dateObj.to && dateObj.from) {
      const toDate = new Date(dateObj.to);
      returnValue = txnDate.getTime() >= fromDate.getTime() && txnDate.getTime() < toDate.getTime();
    } else if (dateObj.from && !dateObj.to) {
      returnValue = dateObj.from.toLowerCase().includes(transaction.date) || txnDate.getTime() >= fromDate.getTime();
    } else {
      returnValue = true;
    }
    return returnValue;
  }

  searchTransations(event: any) {
    this.searchTransations$.next(event.target.value);
    this.cdRef.detectChanges();
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
      this.totalCurrentFunds$ = this.allAccounts$?.pipe(map((data) => data.reduce((a, b) => a + b.balance, 0)));
    }
    this.transactionColumnsObj = structuredClone(this.transactionColumns).reduce((obj, col) => {
      return Object.assign(obj, { [col.name]: col });
    }, {});
  }

  addTransaction() {
    window.scroll(0, 0);
    this.currentMode = Mode.CREATE;
    this.selectedTransaction = {
      transferTransactionId: null,
      transferAccountId: null,
      accountName: '',
      accountId: '',
      budgetId: this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget)?.id!,
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
    const account = this.ngxsStore
      .selectSnapshot(AccountsState.getAllAccounts)
      .find((acc) => acc.id! === transaction.accountId);
    if (account) {
      this.selectedAccount = account;
    }
    const payee = this.ngxsStore
      .selectSnapshot(PayeesState.getAllPayees)
      .find((payee) => payee.id! === transaction.payeeId);
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
    } catch (err) {}
  }

  async createNewPayee() {
    const payee: Payee = {
      name: this.searchPayee$.value,
      budgetId: this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget)?.id!,
      transferAccountId: null,
      deleted: false,
      createdAt: new Date().toISOString(),
    };
    // this.ngxsStore.dispatch(new PayeesActions.CreatePayee(payee));
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
      const allPayees = this.ngxsStore.selectSnapshot(PayeesState.getAllPayees);
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
      const budgetAccounts = this.ngxsStore.selectSnapshot(AccountsState.getBudgetAccounts);
      const isSelectedAccBudget = budgetAccounts.find((acc) => acc.id! === this.selectedPayee?.transferAccountId!);
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
    if (this.selectedTransaction) {
      this.selectedTransaction.categoryId = category.id!;
      this.selectedTransaction.categoryName = category.name;
    }
    this.searchCategory$.next(category.name);
    this.closeCategorySelectMenu();
  }

  deleteTransaction(transaction: NormalizedTransaction) {
    this.dbService.deleteTransaction(transaction.id!);
    if (transaction.transferTransactionId) {
      this.dbService.deleteTransaction(transaction.transferTransactionId);
    }
  }

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
          this.createNewTransaction(amount, this.selectedTransaction, this.selectedAccount, this.selectedPayee);
          this.cancelTransactionSave();
          break;
        case Mode.EDIT:
          const transactions = this.ngxsStore.selectSnapshot(TransactionsState.getAllTransactions);
          let existingTransaction = transactions.find((tran) => tran.id === this.selectedTransaction?.id);
          // console.log('selected payee:', this.selectedPayee);
          // console.log('selected account:', this.selectedAccount);
          // console.log('selected transaction:', this.selectedTransaction);
          // console.log('exisiting transaction:', existingTransaction);
          // console.log(
          //   'transfer transaction:',
          //   this.store.transactions$.value.find((tran) => tran.id! === existingTransaction?.transferTransactionId)
          // );
          if (
            this.selectedTransaction.payeeId !== existingTransaction?.payeeId &&
            this.selectedTransaction.accountId !== existingTransaction?.accountId
          ) {
            // changing payees and accounts
            // this is a completely new transaction, delete both existingTransaction and existing transferTransaction
            this.dbService.deleteTransaction(existingTransaction?.id!);
            if (existingTransaction?.transferTransactionId) {
              this.dbService.deleteTransaction(existingTransaction.transferTransactionId);
            }
            this.createNewTransaction(amount, this.selectedTransaction, this.selectedAccount, this.selectedPayee);
          } else {
            // just update the info of selected transaction and transfer if it exists, no need for all the logic
            // update selected transaction
            const editData = {
              id: this.selectedTransaction.id!,
              amount,
              date: this.selectedTransaction.date,
              payeeId: this.selectedTransaction.payeeId,
              accountId: this.selectedTransaction.accountId,
              categoryId: this.selectedTransaction.categoryId,
              note: this.selectedTransaction.note,
              transferTransactionId:
                this.selectedPayee.transferAccountId && existingTransaction?.transferTransactionId
                  ? existingTransaction.transferTransactionId
                  : null,
              transferAccountId:
                this.selectedPayee.transferAccountId && existingTransaction?.transferTransactionId
                  ? this.selectedPayee.transferAccountId
                  : null,
            };
            this.dbService.editTransaction(editData);
            if (existingTransaction?.transferTransactionId) {
              const transferTransaction = transactions.find(
                (tran) => tran.id! === existingTransaction?.transferTransactionId,
              );
              if (transferTransaction) {
                const isDeleteTransfer = this.selectedPayee.transferAccountId ? false : true;
                if (isDeleteTransfer) {
                  this.dbService.deleteTransaction(transferTransaction.id!);
                } else {
                  const transferEditData = {
                    id: transferTransaction.id!,
                    amount: -amount,
                    payeeId: this.selectedAccount.transferPayeeId!,
                    accountId: this.selectedPayee.transferAccountId!,
                    categoryId: null, // will always be null, check handleTransferTransaction method for details
                    transferTransactionId: transferTransaction.transferTransactionId,
                    transferAccountId: this.selectedAccount.id!,
                    date: this.selectedTransaction.date,
                    note: this.selectedTransaction.note ?? '',
                  };
                  this.dbService.editTransaction(transferEditData);
                }
              }
            } else {
              if (this.selectedPayee.transferAccountId) {
                // if no transfer transaction exists but new payee is transfer, create a new transfer transaction
                this.handleTransferTransaction(
                  -amount,
                  this.selectedTransaction,
                  this.selectedPayee,
                  this.selectedAccount,
                );
              }
            }
          }
          this.cancelTransactionSave();
          break;
      }
    }
  }

  async createNewTransaction(
    amount: number,
    selectedTransaction: NormalizedTransaction,
    selectedAccount: Account,
    selectedPayee: Payee,
  ) {
    const newTransaction: Transaction = {
      budgetId: this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget)?.id!,
      date: selectedTransaction.date,
      amount: amount,
      accountId: selectedTransaction.accountId,
      payeeId: selectedTransaction.payeeId,
      categoryId: selectedTransaction.categoryId,
      note: selectedTransaction.note ?? '',
      source: 'pennywise',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      deleted: false,
    };
    const createdTransac = await this.dbService.createTransaction(newTransaction);
    selectedTransaction.id = createdTransac.id;
    if (selectedPayee?.transferAccountId) {
      // this is a transfer payee, create another transaction
      this.handleTransferTransaction(-amount, selectedTransaction, selectedPayee, selectedAccount);
    }
  }

  /**
   * @description
   * - Handles creation of transfer transaction, the transaction that is associated with transfer payee
   * - It also edits the current transaction with the updated data
   * @param {number} amount the amount for this transaction (make sure amount is negative for outflow)
   * @param {NormalizedTransaction} selectedTransaction the current transaction being created/edited
   * @param {Payee} selectedPayee the selected payee of current transaction, it should have the transferAccountId
   * @param {Account} selectedAccount the selected account of the current transaction
   */
  async handleTransferTransaction(
    amount: number,
    selectedTransaction: NormalizedTransaction,
    selectedPayee: Payee,
    selectedAccount: Account,
  ) {
    const transferTransac: Transaction = {
      budgetId: this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget)?.id!,
      amount: amount,
      accountId: selectedPayee.transferAccountId!,
      payeeId: selectedAccount.transferPayeeId!,
      transferTransactionId: selectedTransaction.id!,
      transferAccountId: selectedAccount.id!,
      date: selectedTransaction.date,
      // @NOTE: for now tranfer transaction won't have a category since I can only transfer between budget accounts or from budget to tracking accounts
      // transactions cannot be created from tracking accounts, only to them from budget accounts, either inflow or outflow
      categoryId: null,
      note: selectedTransaction.note ?? '',
      source: 'pennywise',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      deleted: false,
    };
    const transferCreatedTransac = await this.dbService.createTransaction(transferTransac);
    const editData = {
      id: selectedTransaction.id!,
      payeeId: selectedPayee.id!,
      accountId: selectedAccount.id!,
      amount: -amount,
      transferTransactionId: transferCreatedTransac.id,
      transferAccountId: selectedPayee.transferAccountId!,
      categoryId: selectedTransaction.categoryId,
      note: selectedTransaction.note ?? '',
      date: selectedTransaction.date,
    };
    await this.dbService.editTransaction(editData);
  }

  trackByTransactionId(index: number, transaction: NormalizedTransaction) {
    return transaction.id!;
  }
}
