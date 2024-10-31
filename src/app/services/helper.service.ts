import { Injectable } from '@angular/core';
import { Transaction } from '../models/transaction.model';
import { Category } from '../models/category.model';
import { Dropdown, DropdownOptions } from 'flowbite';
import { CategoryGroupData } from '../models/state.model';
import { Account } from '../models/account.model';

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

  constructor() {}

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
    categoryGroupData: CategoryGroupData[]
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

  isCategoryCreditCard(category: Category) {
    if (category?.name?.toLowerCase().includes('credit')) {
      return true;
    }
    return false;
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
    options: DropdownOptions = this.defaultOptions
  ): Dropdown {
    const targetEl = document.getElementById(`${targetIdPrefix}-${id}`);
    const triggerEl = document.getElementById(`${triggerIdPrefix}-${id}`);
    const dropdown = new Dropdown(targetEl, triggerEl, options);
    return dropdown;
  }
}
