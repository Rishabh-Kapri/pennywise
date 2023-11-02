import { Component, Input, OnChanges, SimpleChanges } from '@angular/core';
import { Account } from '../models/account.model';
import { StoreService } from '../services/store.service';
import { map, switchMap } from 'rxjs/operators';
import { BehaviorSubject, Observable, combineLatest, of } from 'rxjs';
import { Category } from '../models/category.model';
import { NormalizedTransaction, Transaction, TransactionColumns } from '../models/transaction.model';
import { Dropdown } from 'flowbite';
import { HelperService } from '../services/helper.service';
import { CategoryGroupData } from '../models/state.model';
import { Payee } from '../models/payee.model';
import { DatabaseService } from '../services/database.service';

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
export class TransactionsComponent implements OnChanges {
  @Input() account: Account | null;
  mode = Mode;
  totalCurrentFunds$: Observable<number>;
  categoryObj$: Observable<Record<string, Category>>;

  transactionColumns: Array<{ name: string; class: string }> = [];
  transactionColumnsObj: Record<string, { name: string; class: string }>;
  editingTransaction: NormalizedTransaction;
  selectedTransaction: NormalizedTransaction;
  currentMode: Mode;

  categorySelectDropdown: Dropdown;
  payeeSelectDropdown: Dropdown;
  categoryGroupData$: Observable<CategoryGroupData[]>;
  payeesData$: Observable<PayeesData>;
  searchCategory$ = new BehaviorSubject<string>('');
  searchPayee$ = new BehaviorSubject<string>('');

  constructor(public store: StoreService, public helperService: HelperService, private dbService: DatabaseService) {
    this.categoryGroupData$ = combineLatest([
      this.store.categoryGroupData$,
      this.store.inflowCategory$,
      this.searchCategory$,
    ]).pipe(
      switchMap(([categoryGroupData, inflowCategory, search]) => {
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
        return of(data);
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
          if (payee.name.toLowerCase().includes(searchStr)) {
            if (payee.transferAccountId) {
              payeesData.Transfers.push(payee);
            } else {
              payeesData.Saved.push(payee);
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
    if (changes['account']) {
      this.setAccountData();
    }
  }

  searchCategory(event: any) {
    this.searchCategory$.next(event.target.value);
  }

  searchPayee(event: any) {
    this.searchPayee$.next(event.target.value);
  }

  setAccountData() {
    this.transactionColumns = [];
    if (this.account) {
      const cols = structuredClone(TransactionColumns);
      this.transactionColumns = cols.filter((col) => col.name !== 'Account');
      this.totalCurrentFunds$ = of(this.account.balance);
    } else {
      const cols = structuredClone(TransactionColumns);
      this.transactionColumns = cols;
      this.totalCurrentFunds$ = this.store.accounts$?.pipe(map((data) => data.reduce((a, b) => a + b.balance, 0)));
    }
    this.transactionColumnsObj = structuredClone(this.transactionColumns).reduce((obj, col) => {
      return Object.assign(obj, { [col.name]: col });
    }, {});
  }

  addTransaction() {
    this.currentMode = Mode.CREATE;
    this.selectedTransaction = {
      accountName: '',
      accountId: '',
      budgetId: this.store.selectedBudet,
      date: '',
      outflow: 0,
      inflow: 0,
      balance: 0,
      payeeName: '',
      payeeId: '',
      categoryName: '',
      categoryId: '',
    };
  }

  selectTransaction(transaction: NormalizedTransaction) {
    this.currentMode = Mode.SELECT;
    this.selectedTransaction = transaction;
    // this.search$.next(transaction.categoryName);
  }

  openDatePicker(transaction: Transaction) {
    const datePickerEl = document.getElementById(`datePicker-${transaction.id}`);
    new Datepicker(datePickerEl, {});
  }

  changeTransactionDate(date: string, transaction: NormalizedTransaction) {
    console.log(date, transaction);
  }

  showCategorySelectMenu(transaction: NormalizedTransaction) {
    this.categorySelectDropdown = this.helperService.getDropdownInstance(
      transaction.id!,
      'categorySelectDropdown',
      'categorySelectBtn'
    );
    setTimeout(() => {
      const categoryInput = document.getElementById(`categorySelectInput-${transaction.id}`) as HTMLInputElement;
      categoryInput.focus();
      this.categorySelectDropdown.show();
    });
  }

  showPayeeSelectMenu(transaction: NormalizedTransaction) {
    console.log('opening payee select menu', this.payeeSelectDropdown);
    if (this.payeeSelectDropdown) {
      this.payeeSelectDropdown.toggle();
      return;
    }
    this.payeeSelectDropdown = this.helperService.getDropdownInstance(
      transaction.id!,
      'payeeSelectDropdown',
      'payeeSelectBtn'
    );
    this.payeeSelectDropdown.toggle();
    setTimeout(() => {
      const payeeInput = document.getElementById(`payeeSelectInput-${transaction.id}`) as HTMLInputElement;
      // payeeInput.focus();
    });
  }

  closeCategorySelectMenu(transaction: NormalizedTransaction) {
    if (this.categorySelectDropdown) {
      this.categorySelectDropdown.hide();
    }
    // this.categorySelectDropdown = this.helperService.getDropdownInstance(
    //   transaction.id!,
    //   'categorySelectDropdown',
    //   'categorySelectBtn'
    // );
    // this.categorySelectDropdown.toggle();
  }

  closePayeeSelectMenu() {
    if (this.payeeSelectDropdown) {
      this.payeeSelectDropdown.hide();
    }
  }

  changeAmount(field: 'inflow' | 'outflow', value: number) {
    // @TODO
    // if inflow is selected reset outflow, and vice versa
    // also normalize calculations
  }

  async createNewPayee() {
    const payee: Payee = {
      name: this.searchPayee$.value,
      budgetId: this.store.selectedBudet,
      transferAccountId: null,
      deleted: false,
      createdAt: new Date().toISOString(),
    };
    console.log(payee);
    await this.dbService.createPayee(payee);
  }

  selectPayee(payee: Payee) {
    console.log('payee selected', payee);
  }
}
