import { Pipe, PipeTransform } from '@angular/core';
import { Category } from '../models/category.model';
import { Store } from '@ngxs/store';
import { TransactionsState } from '../store/dashboard/states/transactions/transactions.state';
import { AccountsState } from '../store/dashboard/states/accounts/accounts.state';
import { NormalizedTransaction } from '../models/transaction.model';
import { HelperService } from '../services/helper.service';

@Pipe({
  name: 'transactionCount',
  standalone: false,
  pure: true,
})
export class TransactionCountPipe implements PipeTransform {
  constructor(
    private ngxsStore: Store,
    private helperService: HelperService,
  ) { }

  transform(category: Category, month: string): number {
    const allTransactions = this.ngxsStore.selectSnapshot(TransactionsState.getNormalizedTransaction);
    const ccAccounts = this.ngxsStore.selectSnapshot(AccountsState.getCreditCardAccounts);
    if (category.id === '404f1661-caee-496a-9f53-a7981f74397d') {
      console.log(category);
      console.log(month, allTransactions);
    }

    let categoryTransactions: NormalizedTransaction[] = [];
    if (this.helperService.isCategoryCreditCard(category)) {
      categoryTransactions = this.helperService.getTransactionsForAccount(allTransactions, [
        ...ccAccounts.map((acc) => acc.id!),
      ]) as NormalizedTransaction[];
    } else {
      if (category.id === '404f1661-caee-496a-9f53-a7981f74397d') console.log('fetching transactions for category');
      categoryTransactions = this.helperService.getTransactionsForCategory(allTransactions, [category.id!]);
      if (category.id === '404f1661-caee-496a-9f53-a7981f74397d') console.log(categoryTransactions);
    }
    const filteredTransactions = this.helperService.filterTransactionsBasedOnMonth(categoryTransactions, month);
    if (category.id === '404f1661-caee-496a-9f53-a7981f74397d') console.log(filteredTransactions);

    return filteredTransactions.length;
  }
}
