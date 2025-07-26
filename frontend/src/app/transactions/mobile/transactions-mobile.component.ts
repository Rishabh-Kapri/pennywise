import { Component, ChangeDetectionStrategy } from '@angular/core';
import { Store } from '@ngxs/store';
import { Observable, map } from 'rxjs';
import { NormalizedTransaction } from 'src/app/models/transaction.model';
import { TransactionsState } from 'src/app/store/dashboard/states/transactions/transactions.state';
import { HelperService } from 'src/app/services/helper.service';

interface GroupedTransactions {
  month: string; // e.g. 'June 2024'
  dates: {
    date: string; // e.g. 'Saturday, Jun 21'
    transactions: NormalizedTransaction[];
  }[];
}

@Component({
  selector: 'app-transactions-mobile',
  templateUrl: './transactions-mobile.component.html',
  styleUrls: ['./transactions-mobile.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TransactionsMobileComponent {
  groupedTransactions$: Observable<GroupedTransactions[]>;

  constructor(private store: Store, private helper: HelperService) {
    this.groupedTransactions$ = this.store.select(TransactionsState.getNormalizedTransaction).pipe(
      map(transactions => this.groupByMonthAndDate(transactions))
    );
  }

  private groupByMonthAndDate(transactions: NormalizedTransaction[]): GroupedTransactions[] {
    if (!transactions || transactions.length === 0) return [];
    // Sort by date descending
    const monthMap = new Map<string, Map<string, NormalizedTransaction[]>>();
    for (const txn of transactions) {
      const dateObj = new Date(txn.date);
      const monthKey = dateObj.toLocaleString('default', { month: 'long', year: 'numeric' }); // e.g. 'June 2024'
      const dateKey = dateObj.toLocaleDateString('en-US', { weekday: 'long', month: 'short', day: 'numeric' }); // e.g. 'Saturday, Jun 21'
      if (!monthMap.has(monthKey)) monthMap.set(monthKey, new Map());
      const dateMap = monthMap.get(monthKey)!;
      if (!dateMap.has(dateKey)) dateMap.set(dateKey, []);
      dateMap.get(dateKey)!.push(txn);
    }
    // Convert to array
    return Array.from(monthMap.entries()).map(([month, dateMap]) => ({
      month,
      dates: Array.from(dateMap.entries()).map(([date, transactions]) => ({ date, transactions }))
    }));
  }

  trackByMonth(index: number, group: GroupedTransactions) { return group.month; }
  trackByDate(index: number, dateGroup: {date: string}) { return dateGroup.date; }
  trackByTxn(index: number, txn: NormalizedTransaction) { return txn.id; }
} 