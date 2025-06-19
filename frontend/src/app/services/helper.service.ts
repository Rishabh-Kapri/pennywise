import { Injectable } from '@angular/core';
import { Transaction } from '../models/transaction.model';
import { Category } from '../models/category.model';
import { Dropdown, DropdownOptions } from 'flowbite';
import { CategoryGroupData } from '../models/state.model';
import { Account } from '../models/account.model';
import { Store } from '@ngxs/store';
import { TransactionsState } from '../store/dashboard/states/transactions/transactions.state';
import { AccountsState } from '../store/dashboard/states/accounts/accounts.state';
import { categoryGroups } from 'src/assets/mock-data';

@Injectable({
  providedIn: 'root',
})
export class HelperService {
  defaultOptions: DropdownOptions = {
    placement: 'bottom',
    triggerType: 'click',
    offsetSkidding: 0,
    offsetDistance: 10,
    delay: 300,
    ignoreClickOutsideClass: false,
  };

  constructor(private ngxsStore: Store) {}

  keyValuePipeOriginalOrder() {
    return 0;
  }

  /**
   * This sets
   */
  setNextMonthCategoryBalance() {}

  compareMonthKeyWithCurrentMonthKey(key: string) {
    const { year, month } = this.splitKeyIntoMonthYear(key);
    const date = new Date(year, month);
    const currentDate = new Date();
    const currentYear = currentDate.getFullYear();
    const currentMonth = currentDate.getMonth();
    return date < new Date(currentYear, currentMonth);
  }

  getPreviousMonthKey(currentKey: string): string {
    const { year, month } = this.splitKeyIntoMonthYear(currentKey);
    const date = new Date(year, month - 1);
    const previousKey = `${date.getFullYear()}-${date.getMonth()}`;
    return previousKey;
  }

  splitKeyIntoMonthYear(key: string) {
    const selectedDate = key.split('-');
    const year = parseInt(selectedDate[0], 10);
    const month = parseInt(selectedDate[1], 10);
    return { year, month };
  }

  /**
   * Returns the month key in format yyyy-mm
   */
  getCurrentMonthKey(): string {
    const date = new Date();
    return `${date.getFullYear()}-${date.getMonth()}`;
  }

  /**
   * Returns the date in the format YYYY-MM-DD
   * @Param{number} monthDiff optional diff for month, default is zero
   */
  getDateInStringFormat(dateObj: Date, monthDiff: number = 0): string {
    dateObj.setDate(1);
    dateObj.setMonth(dateObj.getMonth() + monthDiff);
    const date = dateObj.getDate();
    const month = dateObj.getMonth() + 1;
    return `${dateObj.getFullYear()}-${month}-${date}`;
  }

  getCurrentMonthDateRange() {
    const startDate = this.getDateInStringFormat(new Date());
    const endDate = this.getDateInStringFormat(new Date(), 1);
    return { startDate, endDate };
  }

  /**
   * Returns the startDate and endDate array between two dates.
   * @Param{string} startDate the date to start with, should be in format YYYY-MM-DD
   * @Param{string} endDate the date to end with (non-inclusive), should be in format YYYY-MM-DD
   * @example
   * startDate = "2025-01-01"
   * endDate = "2025-03-01"
   * const range = getDateRange(startDate, endDate)
   * console.log(range)
   * [
       { startDate: '2025-1-1', endDate: '2025-2-1', monthKey: '2025-0' },
       { startDate: '2025-2-1', endDate: '2025-3-1', monthKey: '2025-1' },
       { startDate: '2025-3-1', endDate: '2025-4-1', monthKey: '2025-2' }
   * ]
   */
  getDateRange(startDate: string, endDate: string): { startDate: string; endDate: string; monthKey: string }[] {
    const arr: { startDate: string; endDate: string; monthKey: string }[] = [];

    let startDateStr = startDate;
    let endDateStr = endDate;
    let startDateObj = new Date(startDateStr);
    const endDateObj = new Date(endDateStr);

    while (startDateObj.getTime() !== endDateObj.getTime()) {
      const dateStrIncrement = this.getDateInStringFormat(startDateObj, 1);
      arr.push({
        startDate: startDateStr,
        endDate: dateStrIncrement,
        monthKey: `${startDateObj.getFullYear()}-${startDateObj.getMonth()-1}`,
      });
      startDateStr = dateStrIncrement;
      startDateObj = new Date(startDateStr);
    }
    return arr;
  }

  /**
   * Returns the current date using new Date() in format dd/mm/yyy
   */
  getDateInDbFormat() {
    const date = new Date();
    return `${date.getDate()}/${date.getMonth() + 1}/${date.getFullYear()}`;
  }

  /**
   * Currently parses DD/MM/YYY date format to MM/DD/YYYY
   * @TODO In future if needed use moment.js for more customizability
   */
  getDateFormatToParse(dateString: string) {
    const splitDate = dateString.split('/');
    return `${splitDate[1]}/${splitDate[0]}/${splitDate[2]}`;
  }

  /**
   * Get transactions for the category group
   */
  getTransactionForCategoryGroup(
    transactions: Transaction[],
    categoryGroupId: string,
    categoryGroupData: CategoryGroupData[],
  ): Transaction[] {
    let groupTransactions: Transaction[] = [];
    let categoryIds: string[] = [];
    // get categories for the groupId
    const groupData = categoryGroupData.find((group) => group.id === categoryGroupId);
    if (groupData) {
      categoryIds = groupData.categories.map((cat) => cat.id!);
      groupTransactions = this.getTransactionsForCategory(transactions, categoryIds);
    }
    return groupTransactions;
  }

  isCategoryCreditCard(category: Category): boolean {
    return category?.name?.toLowerCase().includes('credit');
  }

  /**
   * Filters out transactions for specified category ids
   * @param {Transaction[]} transactions the transactions from which to filter
   * @param {string[]} categoryIds the ids of the categories
   * @returns {Transaction[]} filtered transactions
   */
  getTransactionsForCategory(transactions: Transaction[], categoryIds: string[]): Transaction[] {
    return transactions.filter((transaction) => categoryIds.includes(transaction.categoryId ?? ''));
  }

  getTransactionsForAccount(transactions: Transaction[], accountIds: string[], shouldConsole: boolean = false) {
    if (shouldConsole) {
      console.log('getTransactionsForAccount:', transactions, accountIds);
      console.log(transactions.filter((txn) => accountIds.includes(txn.accountId ?? '')));
    }
    return transactions.filter((transaction) => accountIds.includes(transaction.accountId ?? ''));
  }

  filterTransactionsReport(
    transactions: Transaction[],
    categoryIds: string[],
    accountIds: string[],
    startDateStr: string,
    endDateStr: string,
  ) {
    const startDate = new Date(startDateStr);
    const endDate = new Date(endDateStr);
    const filteredTxnsDate = transactions.filter((txn) => {
      const date = new Date(txn.date);
      return date.getTime() >= startDate.getTime() && date.getTime() < endDate.getTime();
    });
    // when selecting all categories or accounts the following function calls can be skipped
    const categoryTxns = this.getTransactionsForCategory(filteredTxnsDate, categoryIds);
    const accountTxns = this.getTransactionsForAccount(categoryTxns, accountIds);
    return accountTxns;
  }

  /**
   * Get the activity amount for category based on the transactions
   * @param {Transaction[]} transactions the transactions from which to filter
   * @param {Category} category the category to get activity for
   */
  getActivityForCategory(transactions: Transaction[], category: Category, accounts: Account[]): number {
    let filteredTransactions: Transaction[] = [];
    if (this.isCategoryCreditCard(category)) {
      // if category is credit card then fetch accountId
      const ccAccount = accounts.find((acc) => acc.name.toLowerCase().includes('credit'));
      if (ccAccount) {
        filteredTransactions = this.getTransactionsForAccount(transactions, [ccAccount.id!]);
      }
    } else {
      filteredTransactions = this.getTransactionsForCategory(transactions, [category.id!]);
    }
    return filteredTransactions.reduce((acc, curr) => acc + curr.amount, 0);
  }

  reduceCategoriesAmount(categories: Category[], key: 'balance' | 'activity' | 'budgeted', monthKey: string) {
    return categories.reduce((amount, cat) => amount + (cat?.[key]?.[monthKey] ?? 0), 0);
  }

  /**
   * Filter the transactions for the monthKey provided from the transactions
   * @param {Transaction[]} transactions the transactions to filter from
   * @param {string} monthKey the selected month key
   * @returns Transaction[] the filtered transaction for the provided month
   */
  filterTransactionsBasedOnMonth(transactions: Transaction[], monthKey: string, categoryId?: string): Transaction[] {
    const { year, month } = this.splitKeyIntoMonthYear(monthKey);
    const startDate = new Date(year, month);
    const endDate = new Date(year, month + 1);
    const filteredTransactions = transactions.filter((transaction) => {
      const date = new Date(transaction.date);
      return date.getTime() >= startDate.getTime() && date.getTime() < endDate.getTime();
      // return (
      //   date.getTime() >= startDate.getTime() &&
      //   date.getTime() < endDate.getTime() &&
      //   (!categoryId || transaction.categoryId === categoryId)
      // );
    });
    return filteredTransactions;
  }

  /**
   * Generic method that returns a dropdown instance
   */
  getDropdownInstance(
    id: string,
    targetIdPrefix: string,
    triggerIdPrefix: string,
    options: DropdownOptions = this.defaultOptions,
  ): Dropdown {
    const targetEl = document.getElementById(`${targetIdPrefix}-${id}`);
    const triggerEl = document.getElementById(`${triggerIdPrefix}-${id}`);
    const dropdown = new Dropdown(targetEl, triggerEl, options);
    return dropdown;
  }

  sumTransaction(transactions: Transaction[]) {
    return transactions.reduce((acc, curr) => acc + curr.amount, 0);
  }

  /**
   * @description Fetches the category's balance
   * @param {string} monthKey the key of the month
   * @param {Category} category the category for which balance is to be fetched
   * @param {Transaction[]} currentTransactions the category transactions for monthKey
   */
  getCategoryBalance(monthKey: string, category: Category, currentTransactions: Transaction[]): number {
    let balance = 0;
    const isCategoryCreditCard = this.isCategoryCreditCard(category);
    const previousMonthKey = this.getPreviousMonthKey(monthKey);
    if ((category.budgeted[previousMonthKey] === undefined && !isCategoryCreditCard) || previousMonthKey === '2021-5') {
      // if previous month budgeted doesn't exists, use the current month budgeted
      // even if no money is assigned, 0 will be present as budgeted even if one category is budgeted
      balance = (category.budgeted?.[monthKey] ?? 0) + this.sumTransaction(currentTransactions);
    } else {
      // if previous month has money budgeted
      if (category.balance?.[previousMonthKey] === undefined) {
        // if no balance is calculated for pervious month then calculate it
        const previousMonthKeyTransactions = this.filterTransactionsBasedOnMonth(
          this.ngxsStore.selectSnapshot(TransactionsState.getAllTransactions),
          previousMonthKey,
        );
        let catPreviousMonthTransactions: Transaction[] = [];
        const ccAccounts = this.ngxsStore.selectSnapshot(AccountsState.getCreditCardAccounts);

        if (isCategoryCreditCard) {
          catPreviousMonthTransactions = this.getTransactionsForAccount(previousMonthKeyTransactions, [
            ...ccAccounts.map((acc) => acc.id!),
          ]);
        } else {
          catPreviousMonthTransactions = this.getTransactionsForCategory(previousMonthKeyTransactions, [category.id!]);
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
}
