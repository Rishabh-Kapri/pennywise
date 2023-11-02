import { Injectable } from '@angular/core';
import { Transaction } from '../models/transaction.model';
import { Category } from '../models/category.model';
import { Dropdown, DropdownOptions } from 'flowbite';

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

  getCurrentMonthKey(): string {
    const date = new Date();
    return `${date.getFullYear()}-${date.getMonth()}`;
  }

  /**
   * Get the activity amount for category based on the transactions
   */
  getActivityForCategory(transactions: Transaction[], category: Category): number {
    return transactions
      .filter((transaction) => transaction.categoryId === category.id)
      .reduce((acc, curr) => acc + curr.amount, 0);
  }

  /**
   * Filter the transactions for the monthKey provided from all the transactions
   */
  filterTransactionsBasedOnMonth(allTransactions: Transaction[], monthKey: string): Transaction[] {
    const { year, month } = this.splitKeyIntoMonthYear(monthKey);
    const startDate = new Date(year, month);
    const endDate = new Date(year, month + 1);
    const filteredTransactions = allTransactions.filter((transaction) => {
      const date = new Date(Date.parse(transaction.date));
      return date.getTime() >= startDate.getTime() && date.getTime() < endDate.getTime();
    });
    return filteredTransactions;
  }

  /**
   * Generic method for dropdown
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
