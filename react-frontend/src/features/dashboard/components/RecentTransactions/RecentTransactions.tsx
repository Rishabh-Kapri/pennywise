import { Link } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { selectRecentTransactions } from '../../store/dashboardSlice';
import { ArrowDownLeft, ArrowUpRight, Receipt } from '@phosphor-icons/react';
import { formatCurrency } from '../../utils';
import styles from './RecentTransactions.module.css';

const formatShortDate = (dateStr: string): string => {
  const date = new Date(dateStr);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);

  if (date.toDateString() === today.toDateString()) return 'Today';
  if (date.toDateString() === yesterday.toDateString()) return 'Yesterday';

  return date.toLocaleDateString('en-IN', {
    month: 'short',
    day: 'numeric',
  });
};

export default function RecentTransactions() {
  const transactions = useAppSelector(selectRecentTransactions);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2 className={styles.title}>
          <Receipt size={18} />
          Recent transactions
        </h2>
        <Link to="/transactions" className={styles.seeAll}>
          See all
        </Link>
      </div>

      {transactions.length > 0 ? (
        <div className={styles.list}>
          {transactions.map((txn) => {
            const isInflow = (txn.inflow ?? 0) > 0;
            const amount = isInflow ? txn.inflow : txn.outflow;

            return (
              <div key={txn.id} className={styles.row}>
                <span className={`${styles.directionIcon} ${isInflow ? styles.inflowIcon : ''}`}>
                  {isInflow ? <ArrowDownLeft size={16} /> : <ArrowUpRight size={16} />}
                </span>
                <div className={styles.rowBody}>
                  <span className={styles.payee}>{txn.payeeName || 'Unknown payee'}</span>
                  <span className={styles.category}>{txn.categoryName || 'Uncategorized'}</span>
                </div>
                <div className={styles.rowMeta}>
                  <span className={`${styles.amount} ${isInflow ? styles.amountInflow : ''}`}>
                    {isInflow ? '+' : '-'}{formatCurrency(amount ?? 0)}
                  </span>
                  <time className={styles.date}>{formatShortDate(txn.date)}</time>
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className={styles.emptyState}>No transactions yet</div>
      )}
    </div>
  );
}
