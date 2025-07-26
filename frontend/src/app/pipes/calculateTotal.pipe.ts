import { Pipe, PipeTransform } from '@angular/core';
import { Amount, CategoryGroupReport, IncomeData } from '../models/reports.model';

/**
 * Calculates total for cateogries, category group or income payees
 * type payee is to be used when calculating total for a payee for all months
 * key needs to be the payee name
 * payeeMonth type is used for calculating the total for all payees for a particular month (payee month column total)
 */
@Pipe({
  name: 'calculateTotal',
  standalone: false,
  pure: true,
})
export class CalculateTotalPipe implements PipeTransform {
  transform(
    txnData: IncomeData[] | CategoryGroupReport,
    type: 'payee' | 'payeeMonth' | 'group' | 'category',
    key: string,
  ) {
    switch (type) {
      case 'payee':
        return this.calculatePayeeTotal(txnData as IncomeData[], key);
      case 'payeeMonth':
        return this.calculateIndiviualPayeeForMonth(txnData as IncomeData[], key);
      case 'group':
        return this.calculateCategoryGroupTotal(txnData as CategoryGroupReport, key);
      case 'category':
        return this.calculateCategoryTotal(txnData as CategoryGroupReport, key);
      default:
        return 0;
    }
  }

  private calculatePayeeTotal(incomeData: IncomeData[], payeeName: string) {
    const amounts = incomeData.find((data) => data.payee === payeeName)?.amounts ?? {};
    return this.sumAmount(amounts);
  }

  private calculateIndiviualPayeeForMonth(incomeData: IncomeData[], monthKey: string) {
    return incomeData.reduce((total, data) => {
      return total + (monthKey === 'all' ? this.sumAmount(data.amounts) : (data.amounts[monthKey] ?? 0));
    }, 0);
  }

  private calculateCategoryGroupTotal(groupData: CategoryGroupReport, monthKey: string) {
    return groupData.categories.reduce((total, cat) => {
      return total + (monthKey === 'all' ? this.sumAmount(cat.amounts) : (cat.amounts[monthKey] ?? 0));
    }, 0);
  }

  private calculateCategoryTotal(groupData: CategoryGroupReport, categoryName: string) {
    const categoryAmounts = groupData.categories.find((cat) => cat.name === categoryName)?.amounts ?? {};
    return this.sumAmount(categoryAmounts);
  }

  private sumAmount(amounts: Amount) {
    return Object.values(amounts).reduce((acc, curr) => acc + curr, 0);
  }
}
