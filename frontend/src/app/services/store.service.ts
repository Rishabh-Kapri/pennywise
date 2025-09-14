import { Injectable } from '@angular/core';
import { FormGroup } from '@angular/forms';
import {
  BehaviorSubject,
  Observable,
  combineLatest,
  concatMap,
  filter,
  from,
  of,
  shareReplay,
  startWith,
  switchMap,
} from 'rxjs';
import { Budget } from '../models/budget.model';
import { CategoryGroup } from '../models/catergoryGroup';
import { Category, InflowCategory } from '../models/category.model';
import { Payee } from '../models/payee.model';
import { Account, BudgetAccountType, TrackingAccountType } from '../models/account.model';
import { DatabaseService } from './database.service';
import { CategoryGroupData, SelectedComponent } from '../models/state.model';
import { INFLOW_CATEGORY_NAME, MASTER_CATEGORY_GROUP_NAME } from '../constants/general';
import { HelperService } from './helper.service';
import { NormalizedTransaction, Transaction } from '../models/transaction.model';
import { getAuth, signInAnonymously, signInWithEmailAndPassword } from 'firebase/auth';
import { Store } from '@ngxs/store';
import { AccountsActions } from '../store/dashboard/states/accounts/accounts.action';
import { BudgetsActions } from '../store/dashboard/states/budget/budget.action';
import { CategoriesActions } from '../store/dashboard/states/categories/categories.action';
import { CategoryGroupsActions } from '../store/dashboard/states/categoryGroups/categoryGroups.action';
import { TransactionsActions } from '../store/dashboard/states/transactions/transaction.action';
import { PayeesActions } from '../store/dashboard/states/payees/payees.action';
import { BudgetsState } from '../store/dashboard/states/budget/budget.state';
import { CategoriesState } from '../store/dashboard/states/categories/categories.state';

@Injectable({
  providedIn: 'root',
})
export class StoreService {
  keys: string[] = [];
  accounts$: BehaviorSubject<Account[]> = new BehaviorSubject<Account[]>([]);
  trackingAccounts$ = new BehaviorSubject<Account[]>([]);
  budgetAccounts$ = new BehaviorSubject<Account[]>([]);
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
  allAccounts$: Observable<Account[]>;
  normalizedTransactions$: Observable<NormalizedTransaction[]>;
  categoryGroupDataWithInflow: any;
  inflowWithBalance$: Observable<InflowCategory | null>;
  collapseAllGroup$ = new BehaviorSubject<boolean>(false);

  accountForm: FormGroup;

  private _selectedComponent = new BehaviorSubject<SelectedComponent>(SelectedComponent.REPORTS);
  private _selectedAccount = new BehaviorSubject<Account | null>(null);
  private _selectedMonth = new BehaviorSubject<string>(this.helperService.getCurrentMonthKey());

  constructor(
    private dbService: DatabaseService,
    private helperService: HelperService,
    private ngxsStore: Store,
  ) {}

  private sumTransaction(transactions: Transaction[]) {
    return transactions.reduce((acc, curr) => acc + curr.amount, 0);
  }

  initStoreActions() {
    const auth = getAuth();
    const email = 'rishabhkapri@gmail.com';
    const pass = 'U7h%QG2$573Nj!@H';
    // signInAnonymously(auth).then().catch();
    signInWithEmailAndPassword(auth, email, pass).then().catch();
    // fetch budget first
    this.ngxsStore.dispatch([new BudgetsActions.GetAllBudgets()]).subscribe({
      next: (res) => {
        console.log('fetched all budgets');
        const selectedBudget = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget);
        const selectedBudgetId = selectedBudget?.id ?? '';
        console.log(selectedBudget, selectedBudgetId);
        this.ngxsStore.dispatch([
          new TransactionsActions.GetAllTransactions(selectedBudgetId),
          new PayeesActions.GetAllPayees(selectedBudgetId),
          new CategoriesActions.GetAllCategories(selectedBudgetId),
          new CategoryGroupsActions.GetAllCategoryGroups(selectedBudgetId),
          new AccountsActions.GetAllAccounts(selectedBudgetId),
        ]);
      },
    });
  }

  async init() {
    const auth = getAuth();
    // when app is first loaded this will fetch the data of the current view
    // get current month's data

    this.budget$ = this.dbService.getBudgetsStream();
    this.budget$
      .pipe(
        concatMap((budgets) => from(budgets)),
        filter((budget) => budget.isSelected === true),
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

    this.allAccounts$ = combineLatest([this.accounts$, this.transactions$]).pipe(
      switchMap(([accounts, transactions]) => {
        for (const acc of accounts) {
          acc.balance = transactions.filter((tran) => tran.accountId === acc.id!).reduce((a, b) => a + b.amount, 0);
        }
        this.budgetAccounts$.next(
          accounts.filter(
            (acc) =>
              [BudgetAccountType.CREDIT_CARD, BudgetAccountType.SAVINGS, BudgetAccountType.CHECKING].includes(
                <BudgetAccountType>acc.type,
              ) && !acc.closed,
          ),
        );
        this.trackingAccounts$.next(
          accounts.filter(
            (acc) =>
              [TrackingAccountType.ASSET, TrackingAccountType.LIABILITY].includes(<TrackingAccountType>acc.type) &&
              !acc.closed,
          ),
        );
        return of(accounts);
      }),
      shareReplay(1),
    );

    // Calculate Normalized Transactions
    this.normalizedTransactions$ = combineLatest([
      this.transactions$.pipe(startWith([])),
      this.allAccounts$.pipe(startWith([])),
      this.payees$.pipe(startWith([])),
      this.categories$.pipe(startWith([])),
      this.selectedAccount$,
    ]).pipe(
      switchMap(([transactions, accounts, payees, categories, selectedAccount]) => {
        const normalizedTransactions: NormalizedTransaction[] = [];
        let prevTransacAmount = 0;
        let accBal = 0;
        transactions = transactions.filter((tran) => !selectedAccount || tran.accountId === selectedAccount.id);
        for (const [index, value] of transactions.entries()) {
          const transaction = value;
          const account = accounts.find((acc) => acc.id === transaction.accountId);
          if (index > 0) {
            accBal = normalizedTransactions[index - 1].balance - prevTransacAmount;
          } else {
            accBal = account?.balance ?? 0;
          }
          const payee = payees.find((payee) => payee.id === transaction.payeeId);
          const transac: NormalizedTransaction = {
            id: transaction.id!,
            budgetId: transaction.budgetId,
            date: transaction.date,
            outflow: transaction.amount < 0 ? Math.abs(transaction.amount) : null,
            inflow: transaction.amount >= 0 ? Math.abs(transaction.amount) : null,
            balance: Number(accBal.toFixed(2)),
            note: transaction.note,
            transferTransactionId: transaction.transferTransactionId ?? null,
            transferAccountId: transaction.transferAccountId ?? null,
            accountName: account?.name ?? '',
            accountId: account?.id!,
            payeeName: payee?.name ?? '',
            payeeId: payee?.id!,
            categoryName: categories.find((cat) => cat.id === transaction.categoryId)?.name ?? null,
            categoryId: transaction.categoryId,
          };
          normalizedTransactions.push(transac);
          prevTransacAmount = transaction.amount;
        }
        return of(normalizedTransactions);
      }),
      shareReplay(1),
    );

    this.categoryGroupData$ = combineLatest([
      this.categoryGroups$.pipe(startWith([])),
      this.categories$,
      this.transactions$.pipe(startWith([])),
      this.selectedMonth$,
      this.collapseAllGroup$,
    ]).pipe(
      switchMap(([categoryGroups, categories, transactions, selectedMonth, collapseAllGroup]) => {
        const categoryGroupData: CategoryGroupData[] = [];
        for (const group of categoryGroups) {
          if (group.name !== MASTER_CATEGORY_GROUP_NAME) {
            const groupCategories = categories.filter((cat) => cat.categoryGroupId === group.id && !cat.hidden);
            const data: CategoryGroupData = {
              categories: [
                ...groupCategories.map((category) => {
                  const currentMonthTransactions = this.helperService.filterTransactionsBasedOnMonth(
                    transactions,
                    selectedMonth,
                  );
                  let currMonthCatTransactions: Transaction[] = [];
                  const ccAccounts = this.accounts$.value.filter((acc) => acc.type === BudgetAccountType.CREDIT_CARD);

                  if (this.helperService.isCategoryCreditCard(category)) {
                    currMonthCatTransactions = this.helperService.getTransactionsForAccount(currentMonthTransactions, [
                      ...ccAccounts.map((acc) => acc.id!),
                    ]) as Transaction[];
                  } else {
                    currMonthCatTransactions = this.helperService.getTransactionsForCategory(currentMonthTransactions, [
                      category.id!,
                    ]);
                    if (category.name === '♪ Spotify') {
                    }
                  }
                  if (category?.budgeted?.[selectedMonth] === undefined) {
                    category.budgeted = { ...category.budgeted, [selectedMonth]: 0 };
                  }
                  category.activity = {
                    ...category.activity,
                    [selectedMonth]: currMonthCatTransactions.reduce((acc, curr) => acc + curr.amount, 0),
                  };
                  // category.balance = this.getCategoryBalanceIterative(
                  //   this.selectedMonth,
                  //   category,
                  //   currMonthCatTransactions
                  // );
                  category.balance = {
                    ...category.balance,
                    [selectedMonth]: this.getCategoryBalance(selectedMonth, category, currMonthCatTransactions),
                  };
                  return { ...category, showBudgetInput: false };
                }),
              ],
              name: group.name,
              id: group.id!,
              balance: {
                [selectedMonth]: groupCategories.reduce((amount, cat) => {
                  return amount + (cat?.balance?.[selectedMonth] ?? 0);
                }, 0),
              },
              activity: {
                [selectedMonth]: groupCategories.reduce((amount, cat) => {
                  return amount + (cat?.activity?.[selectedMonth] ?? 0);
                }, 0),
              },
              budgeted: {
                [selectedMonth]: groupCategories.reduce((amount, cat) => {
                  return amount + (cat?.budgeted?.[selectedMonth] ?? 0);
                }, 0),
              },
              collapsed: collapseAllGroup,
            };
            categoryGroupData.push(data);
          }
        }
        const hiddenCategories = categories.filter((cat) => cat.hidden);
        const hiddenGroup = {
          name: 'Hidden',
          id: `hidden-cat`,
          balance: {
            [selectedMonth]: categories.reduce((amount, cat) => {
              return amount + (cat?.balance?.[selectedMonth] ?? 0);
            }, 0),
          },
          activity: {
            [selectedMonth]: categories.reduce((amount, cat) => {
              return amount + (cat?.activity?.[selectedMonth] ?? 0);
            }, 0),
          },
          budgeted: {
            [selectedMonth]: categories.reduce((amount, cat) => {
              return amount + (cat?.budgeted?.[selectedMonth] ?? 0);
            }, 0),
          },
          collapsed: true,
          categories: [
            ...hiddenCategories.map((category) => {
              const currentMonthTransactions = this.helperService.filterTransactionsBasedOnMonth(
                transactions,
                selectedMonth,
              );
              let currMonthCatTransactions: Transaction[] = [];
              const ccAccounts = this.accounts$.value.filter((acc) => acc.type === BudgetAccountType.CREDIT_CARD);
              if (category.name === '♪ Spotify') {
              }
              if (this.helperService.isCategoryCreditCard(category)) {
                currMonthCatTransactions = this.helperService.getTransactionsForAccount(currentMonthTransactions, [
                  ...ccAccounts.map((acc) => acc.id!),
                ]) as Transaction[];
              } else {
                currMonthCatTransactions = this.helperService.getTransactionsForCategory(currentMonthTransactions, [
                  category.id!,
                ]);
              }
              if (category?.budgeted?.[selectedMonth] === undefined) {
                category.budgeted = { ...category.budgeted, [selectedMonth]: 0 };
              }
              category.activity = {
                ...category.activity,
                [selectedMonth]: currMonthCatTransactions.reduce((acc, curr) => acc + curr.amount, 0),
              };
              // category.balance = this.getCategoryBalanceIterative(
              //   this.selectedMonth,
              //   category,
              //   currMonthCatTransactions
              // );
              category.balance = {
                ...category.balance,
                [selectedMonth]: this.getCategoryBalance(selectedMonth, category, currMonthCatTransactions),
              };
              return { ...category, showBudgetInput: false };
            }),
          ],
        };
        categoryGroupData.push(hiddenGroup);
        return of(categoryGroupData);
      }),
      shareReplay(1),
    );

    this.inflowWithBalance$ = combineLatest([this.inflowCategory$, this.categories$, this.transactions$]).pipe(
      switchMap(([inflowCategory, categories, transactions]) => {
        const categoriesWithoutInflow = categories.filter((cat) => cat.name !== INFLOW_CATEGORY_NAME);
        if (inflowCategory) {
          const totalBudgeted = categoriesWithoutInflow.reduce((totalBudgeted, cat) => {
            return totalBudgeted + Object.values(cat.budgeted).reduce((a, b) => a + b, 0);
          }, 0);
          const inflowAmount = this.helperService
            .getTransactionsForCategory(transactions, [inflowCategory.id!])
            .reduce((totalAmount, transaction) => totalAmount + transaction.amount, 0);
          inflowCategory.budgeted = Number(Number(inflowAmount - totalBudgeted).toFixed(2));
        }
        return of(inflowCategory);
      }),
      shareReplay(1),
    );

    const email = 'rishabhkapri@gmail.com';
    const pass = 'U7h%QG2$573Nj!@H';
    // signInAnonymously(auth).then().catch();
    signInWithEmailAndPassword(auth, email, pass).then().catch();
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
   * @description Fetches the category's balance
   * @param {string} monthKey the key of the month
   * @param {Category} category the category for which balance is to be fetched
   * @param {Transaction[]} currentTransactions the category transactions for monthKey
   */
  getCategoryBalance(monthKey: string, category: Category, currentTransactions: Transaction[]): number {
    let balance = 0;
    const isCategoryCreditCard = this.helperService.isCategoryCreditCard(category);
    const previousMonthKey = this.helperService.getPreviousMonthKey(monthKey);
    if ((category.budgeted[previousMonthKey] === undefined && !isCategoryCreditCard) || previousMonthKey === '2021-5') {
      // if previous month budgeted doesn't exists, use the current month budgeted
      // even if no money is assigned, 0 will be present as budgeted even if one category is budgeted
      balance = (category.budgeted?.[monthKey] ?? 0) + this.sumTransaction(currentTransactions);
    } else {
      // if previous month has money budgeted
      if (category.balance?.[previousMonthKey] === undefined) {
        // if no balance is calculated for pervious month then calculate it
        const previousMonthKeyTransactions = this.helperService.filterTransactionsBasedOnMonth(
          this.transactions$.value,
          previousMonthKey,
        );
        let catPreviousMonthTransactions: Transaction[] = [];
        const ccAccounts = this.accounts$.value.filter((acc) => acc.type === BudgetAccountType.CREDIT_CARD);

        if (isCategoryCreditCard) {
          catPreviousMonthTransactions = this.helperService.getTransactionsForAccount(previousMonthKeyTransactions, [
            ...ccAccounts.map((acc) => acc.id!),
          ]) as Transaction[];
        } else {
          catPreviousMonthTransactions = this.helperService.getTransactionsForCategory(previousMonthKeyTransactions, [
            category.id!,
          ]);
        }
        balance = this.getCategoryBalance(previousMonthKey, category, catPreviousMonthTransactions);
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
        balance =
          (category.balance[previousMonthKey] ?? 0) +
          (category.budgeted[monthKey] ?? 0) +
          this.sumTransaction(currentTransactions);
      } else {
        // if balance is calculated for previous month
        balance =
          category.balance[previousMonthKey] + category.budgeted[monthKey] + this.sumTransaction(currentTransactions);
        category.balance[monthKey] = balance;
      }
    }
    balance = Number(Number(balance).toFixed(2));
    return balance;
  }

  // use a loop instead of recursion
  getCategoryBalanceIterative(monthKey: string, category: Category, currentTransactions: Transaction[]) {
    function sortFunc(a: string, b: string) {
      const value1 = a.split('-');
      const value2 = b.split('-');
      const year1 = +value1[0];
      const month1 = +value1[1];
      const year2 = +value2[0];
      const month2 = +value2[1];
      let returnValue = 0;
      if (year1 === year2) {
        if (month1 >= month2) {
          returnValue = -1;
        } else {
          returnValue = 1;
        }
      } else if (year1 > year2) {
        returnValue = -1;
      } else {
        returnValue = 1;
      }
      return returnValue;
    }
    if (!this.keys.length) {
      this.keys = Object.keys(category.budgeted).sort(sortFunc).reverse();
    }
    for (let index = 0; index < this.keys.length; index++) {
      const key = `${this.keys[index]}`;
      if (category?.balance?.[key] !== undefined) {
        continue;
      }
      const currentMonthTransactions = this.helperService.filterTransactionsBasedOnMonth(this.transactions$.value, key);
      let currMonthCatTransactions: Transaction[] = [];
      const ccAccount = this.accounts$.value.find((acc) => acc.name.toLowerCase().includes('credit'));

      if (this.helperService.isCategoryCreditCard(category)) {
        currMonthCatTransactions = this.helperService.getTransactionsForAccount(currentMonthTransactions, [
          ccAccount?.id!,
        ]) as Transaction[];
      } else {
        currMonthCatTransactions = this.helperService.getTransactionsForCategory(currentMonthTransactions, [
          category.id!,
        ]);
      }
      if (!category.balance) {
        category.balance = {};
      }
      const prevKey = `${this.keys[index - 1]}`;
      const prevBal = prevKey !== 'undefined' ? category.balance[prevKey] : 0;
      const bal = prevBal + category.budgeted[key] + this.sumTransaction(currMonthCatTransactions);
      category.balance[key] = Number(Number(bal).toFixed(2));
      if (key === monthKey) {
        break;
      }
    }
    return category.balance;
  }

  async assignZeroToUnassignedCategories() {
    this.dbService.assignZeroToUnassignedCategories(
      this.ngxsStore.selectSnapshot(CategoriesState.getAllCategories),
      this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth),
    );
  }
}
