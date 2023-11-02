import { Injectable } from '@angular/core';
import { FormGroup } from '@angular/forms';
import { BehaviorSubject, Observable, combineLatest, concatMap, filter, from, of, startWith, switchMap } from 'rxjs';
import { Budget } from '../models/budget.model';
import { CategoryGroup } from '../models/catergoryGroup';
import { Category, InflowCategory } from '../models/category.model';
import { Payee } from '../models/payee.model';
import { Account } from '../models/account.model';
import { DatabaseService } from './database.service';
import { CategoryGroupData, SelectedComponent } from '../models/state.model';
import { INFLOW_CATEGORY_NAME } from '../constants/general';
import { HelperService } from './helper.service';
import { NormalizedTransaction, Transaction } from '../models/transaction.model';

@Injectable({
  providedIn: 'root',
})
export class StoreService {
  accounts$: BehaviorSubject<Account[]> = new BehaviorSubject<Account[]>([]);
  budget$: Observable<Budget[]>;
  categoryGroups$: BehaviorSubject<CategoryGroup[]> = new BehaviorSubject<CategoryGroup[]>([]);
  categories$: BehaviorSubject<Category[]> = new BehaviorSubject<Category[]>([]);
  payees$: BehaviorSubject<Payee[]> = new BehaviorSubject<Payee[]>([]);
  /**
   * All transactions
   */
  transactions$: BehaviorSubject<Transaction[]> = new BehaviorSubject<Transaction[]>([]);
  /**
   * Category group data
   */
  categoryGroupData$: Observable<CategoryGroupData[]>;
  selectedBudget$ = new BehaviorSubject<Budget | null>(null);
  inflowCategory$ = new BehaviorSubject<InflowCategory | null>(null);
  normalizedTransactions$: Observable<NormalizedTransaction[]>;
  categoryGroupDataWithInflow: any;

  accountForm: FormGroup;

  private _selectedComponent = new BehaviorSubject<SelectedComponent>(SelectedComponent.BUDGET);
  private _selectedAccount = new BehaviorSubject<Account | null>(null);
  private _selectedMonth = new BehaviorSubject<string>(this.helperService.getCurrentMonthKey());

  constructor(private dbService: DatabaseService, private helperService: HelperService) {}

  init() {
    // when app is first loaded this will fetch the data of the current view
    // get current month's data
    this.budget$ = this.dbService.getBudgetsStream();
    this.budget$
      .pipe(
        concatMap((budgets) => from(budgets)),
        filter((budget) => budget.isSelected === true)
      )
      .subscribe((budget) => {
        const selectedBudget = budget;
        this.dbService.getAccountsStream(selectedBudget?.id!).subscribe((accounts) => {
          this.accounts$.next(accounts);
        });
        this.dbService.getCategoriesStream(selectedBudget?.id!).subscribe((categories) => {
          this.categories$.next(categories as Category[]);
          const inflowCategory = categories.find((category) => category.name === INFLOW_CATEGORY_NAME);
          if (inflowCategory) {
            this.inflowCategory$.next(inflowCategory as InflowCategory);
          }
        });
        this.dbService.getCategoryGroupsStream(selectedBudget?.id!).subscribe((categoryGroups) => {
          this.categoryGroups$.next(categoryGroups);
        });
        this.dbService.getAllTransactionsStream(selectedBudget?.id!).subscribe((transactions) => {
          this.transactions$.next(transactions);
        });
        this.dbService.getPayeesStream(selectedBudget?.id!).subscribe((payees) => {
          this.payees$.next(payees);
        });
        this.selectedBudget$.next(selectedBudget);
      });

    // calculate normalized transactions
    this.normalizedTransactions$ = combineLatest([
      this.transactions$.pipe(startWith([])),
      this.accounts$.pipe(startWith([])),
      this.payees$.pipe(startWith([])),
      this.categories$.pipe(startWith([])),
      this.selectedAccount$,
      // @TODO: add search subject here
    ]).pipe(
      switchMap(([transactions, accounts, payees, categories, selectedAccount]) => {
        const normalizedTransactions: NormalizedTransaction[] = [];
        let prevBal = 0;
        transactions = transactions.filter((tran) => !selectedAccount || tran.accountId === selectedAccount.id);
        for (const transaction of transactions) {
          const account = accounts.find((acc) => acc.id === transaction.accountId);
          const payee = payees.find((payee) => payee.id === transaction.payeeId);
          const transac: NormalizedTransaction = {
            id: transaction.id!,
            budgetId: transaction.budgetId,
            date: transaction.date,
            outflow: transaction.amount < 0 ? Math.abs(transaction.amount) : null,
            inflow: transaction.amount > 0 ? Math.abs(transaction.amount) : null,
            balance: (account?.balance ?? 0) + prevBal,
            note: transaction.note,
            accountName: account?.name ?? '',
            accountId: account?.id!,
            payeeName: payee?.name ?? '',
            payeeId: payee?.id!,
            categoryName: categories.find((cat) => cat.id === transaction.categoryId)?.name ?? '',
            categoryId: transaction.categoryId,
          };
          normalizedTransactions.push(transac);
          prevBal = transaction.amount;
        }
        return of(normalizedTransactions);
      })
    );

    this.categoryGroupData$ = combineLatest([
      this.categoryGroups$.pipe(startWith([])),
      this.categories$,
      this.transactions$.pipe(startWith([])),
      this.selectedMonth$,
    ]).pipe(
      switchMap(([categoryGroups, categories, transactions, selectedMonth]) => {
        const categoryGroupData: CategoryGroupData[] = [];
        for (const group of categoryGroups) {
          if (group.name !== 'Master Category') {
            const groupCategories = categories.filter((cat) => cat.categoryGroupId === group.id);
            const data: CategoryGroupData = {
              categories: [
                ...groupCategories.map((category) => {
                  if (category.name === 'group 4 - cat 1') {
                    // console.log(transactions);
                  }
                  const currentMonthTransactions = this.helperService.filterTransactionsBasedOnMonth(
                    transactions,
                    this.selectedMonth
                  );
                  if (category?.budgeted?.[selectedMonth] === undefined) {
                    category.budgeted = { ...category.budgeted, [selectedMonth]: 0 };
                  }
                  category.activity = {
                    ...category.activity,
                    [selectedMonth]: this.helperService.getActivityForCategory(currentMonthTransactions, category),
                  };
                  category.balance = {
                    ...category.balance,
                    [selectedMonth]: this.getCategoryBalance(this.selectedMonth, category, currentMonthTransactions),
                  };
                  return { ...category, showBudgetInput: false };
                }),
              ],
              name: group.name,
              id: group.id!,
              balance: groupCategories.reduce((acc, curr) => acc + (curr.balance?.[selectedMonth] ?? 0), 0),
              budgeted: groupCategories.reduce((acc, curr) => acc + (curr.budgeted?.[selectedMonth] ?? 0), 0),
              activity: groupCategories.reduce((acc, curr) => acc + (curr.activity?.[selectedMonth] ?? 0), 0),
            };
            categoryGroupData.push(data);
          }
        }
        return of(categoryGroupData);
      })
    );
  }

  get selectedBudet() {
    return this.selectedBudget$?.value?.id ?? '';
  }

  get selectedComponent$() {
    return this._selectedComponent.asObservable();
  }

  get selectedAccount$() {
    return this._selectedAccount.asObservable();
  }

  get selectedMonth$() {
    return this._selectedMonth.asObservable();
  }

  get selectedMonth() {
    return this._selectedMonth.value;
  }

  set selectedComponent(component: SelectedComponent) {
    this._selectedComponent.next(component);
  }

  set selectedAccount(account: Account | null) {
    this._selectedAccount.next(account);
  }

  set selectedMonth(key: string) {
    this._selectedMonth.next(key);
  }

  getLastSetCategoryMoney(monthKey: string, category: Category, key: 'budgeted' | 'activity' | 'balance'): number {
    // check if month is before than the current month, by current I mean new Date()
    if (this.helperService.compareMonthKeyWithCurrentMonthKey(monthKey)) {
      return 0;
    }
    let money = category?.[key]?.[monthKey];
    if (money === undefined) {
      const previousMonthKey = this.helperService.getPreviousMonthKey(monthKey);
      money = this.getLastSetCategoryMoney(previousMonthKey, category, key);
    }
    money = money ?? 0;
    return money;
  }

  /**
   * Fetches the category's balance
   *
   */
  getCategoryBalance(monthKey: string, category: Category, currentTransactions: Transaction[]): number {
    let balance = 0;
    const previousMonthKey = this.helperService.getPreviousMonthKey(monthKey);
    if (category.budgeted[previousMonthKey] === undefined) {
      // if previous month budgeted doesn't exists, use the current month budgeted
      // even if no money is assigned, 0 will be present as budgeted even if one category is budgeted
      balance =
        (category.budgeted?.[monthKey] ?? 0) + this.helperService.getActivityForCategory(currentTransactions, category);
    } else {
      // if previous month has money budgeted
      if (category.balance?.[previousMonthKey] === undefined) {
        // if no balance is calculated for pervious month then calculate it
        const transactions = this.helperService.filterTransactionsBasedOnMonth(
          this.transactions$.value,
          previousMonthKey
        );
        balance = this.getCategoryBalance(previousMonthKey, category, transactions);
        if (category.balance) {
          category.balance = {
            ...category.balance,
            [previousMonthKey]: balance,
          };
        } else {
          category.balance = {};
          category.balance[previousMonthKey] = balance;
        }
        // calculate current month balance
        if (category.name === 'group 4 - cat 1' && monthKey === '2023-8') {
          // console.log(currentTransactions);
          // console.log('Activity:', this.helperService.getActivityForCategory(currentTransactions, category));
        }
        balance =
          category.balance[previousMonthKey] +
          category.budgeted[monthKey] +
          this.helperService.getActivityForCategory(currentTransactions, category);
      } else {
        // if balance if calculated for previous month
        balance =
          category.balance[previousMonthKey] +
          category.budgeted[monthKey] +
          this.helperService.getActivityForCategory(currentTransactions, category);
        category.balance[monthKey] = balance;
      }
    }
    return balance;
  }

  getLastCategoryMonthBalance(monthKey: string) {
    // fetch transactions for the category
    // fetch budgeted for the category
    // if
  }

  async assignZeroToUnassignedCategories() {
    this.dbService.assignZeroToUnassignedCategories(this.categories$.value, this.selectedMonth);
  }
}
