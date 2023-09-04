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
import { Transaction } from '../models/transaction.model';

@Injectable({
  providedIn: 'root',
})
export class StoreService {
  accounts$: BehaviorSubject<Account[]> = new BehaviorSubject<Account[]>([]);
  budget$: Observable<Budget[]>;
  categoryGroups$: BehaviorSubject<CategoryGroup[]> = new BehaviorSubject<CategoryGroup[]>([]);
  categories$: BehaviorSubject<Category[]> = new BehaviorSubject<Category[]>([]);
  payees$: BehaviorSubject<Payee[]> = new BehaviorSubject<Payee[]>([]);
  transactions$: BehaviorSubject<Transaction[]> = new BehaviorSubject<Transaction[]>([]);
  categoryGroupData$: Observable<CategoryGroupData[]>;
  selectedBudget$ = new BehaviorSubject<Budget | null>(null);
  inflowCategory$ = new BehaviorSubject<InflowCategory | null>(null);

  accountForm: FormGroup;

  private _selectedComponent = new BehaviorSubject<SelectedComponent>(SelectedComponent.BUDGET);
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
        this.dbService
          .getMonthsTransactionsStream(this.selectedMonth, selectedBudget?.id!)
          .subscribe((transactions) => {
            this.transactions$.next(transactions);
          });
        this.selectedBudget$.next(selectedBudget);
      });

    this.categoryGroupData$ = combineLatest([
      this.categoryGroups$.pipe(startWith([])),
      this.categories$,
      this.selectedMonth$,
    ]).pipe(
      switchMap(([categoryGroups, categories, selectedMonth]) => {
        const categoryGroupData: CategoryGroupData[] = [];
        for (const group of categoryGroups) {
          if (group.name !== 'Master Category') {
            const groupCategories = categories.filter((cat) => cat.categoryGroupId === group.id);
            const data: CategoryGroupData = {
              categories: [
                ...groupCategories.map((category) => {
                  const transactions = this.transactions$.value;
                  if (category?.budgeted?.[selectedMonth] === undefined) {
                    category.budgeted = {
                      ...category.budgeted,
                      [selectedMonth]: 0,
                    };
                  }
                  if (category?.activity?.[selectedMonth] === undefined) {
                    category.activity = {
                      ...category.activity,
                      [selectedMonth]: this.helperService.getActivityForCategory(transactions, category),
                    };
                  }
                  if (category?.balance?.[selectedMonth] === undefined) {
                    category.balance = {
                      ...category.balance,
                      [selectedMonth]: this.getCategoryBalance(this.selectedMonth, category, transactions),
                    };
                  }
                  console.log(
                    this.selectedMonth,
                    category.name,
                    'Budgeted:',
                    category.budgeted,
                    'Balance:',
                    category.balance
                  );
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

  get selectedComponent$() {
    return this._selectedComponent.asObservable();
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

  getCategoryBalance(monthKey: string, category: Category, currentTransactions: Transaction[]): number {
    const previousMonthKey = this.helperService.getPreviousMonthKey(monthKey);
    let balance = 0;
    if (category.budgeted[previousMonthKey] === undefined) {
      // if previous month budgeted doesn't exists, use the current monh budgeted
      // even if no money is assigned, 0 will be present as budgeted even if one category is budgeted
      balance =
        (category.budgeted?.[monthKey] ?? 0) - this.helperService.getActivityForCategory(currentTransactions, category);
    } else {
      console.log('getting previous month balance for category:', category.name, category.budgeted, category.balance);
      this.dbService.getMonthsTransactions(previousMonthKey, this.selectedBudget$.value?.id!).then((transactions) => {
        console.log(transactions);
        balance = this.getCategoryBalance(previousMonthKey, category, transactions);
      });
    }
    return balance;
  }

  getLastCategoryMonthBalance(monthKey: string) {
    // fetch transactions for the category
    // fetch budgeted for the category
    // if
  }

  async assignZeroToUnassignedCategories() {
    console.log('SELECTED MONTH:', this.selectedMonth);
    this.dbService.assignZeroToUnassignedCategories(this.categories$.value, this.selectedMonth);
  }
}
