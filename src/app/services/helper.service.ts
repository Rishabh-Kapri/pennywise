import { Injectable } from '@angular/core';
import { Transaction } from '../models/transaction.model';
import { Category } from '../models/category.model';

@Injectable({
  providedIn: 'root',
})
export class HelperService {
  constructor() {}

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

  getActivityForCategory(transactions: Transaction[], category: Category): number {
    return transactions
      .filter((transaction) => transaction.categoryId === category.id)
      .reduce((acc, curr) => acc + curr.amount, 0);
  }
}
