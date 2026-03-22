import { Link } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { selectRecentTransactions } from '../../store/dashboardSlice';
import { Receipt, ArrowUpRight, ArrowDownRight, ChevronRight } from 'lucide-react';
import styles from './RecentTransactions.module.css';

const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(Math.abs(amount));
};

const formatDate = (dateStr: string): string => {
  const date = new Date(dateStr);
  return date.toLocaleDateString('en-IN', {
    day: 'numeric',
    month: 'short',
  });
};

export default function RecentTransactions() {
  const transactions = useAppSelector(selectRecentTransactions);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2 className={styles.title}>
          <Receipt size={20} className={styles.titleIcon} />
          Recent Transactions
        </h2>
        <Link to="/transactions" className={styles.viewAll}>
          View All <ChevronRight size={16} />
        </Link>
      </div>

      {transactions.length > 0 ? (
        <div className={styles.transactionList}>
          {transactions.map((txn) => {
            const isInflow = (txn.inflow ?? 0) > 0;
            const amount = isInflow ? txn.inflow : txn.outflow;

            return (
              <div key={txn.id} className={styles.transactionItem}>
                <div
                  className={`${styles.transactionIcon} ${
                    isInflow ? styles.inflow : styles.outflow
                  }`}
                >
                  {isInflow ? (
                    <ArrowDownRight size={18} />
                  ) : (
                    <ArrowUpRight size={18} />
                  )}
                </div>
                <div className={styles.transactionDetails}>
                  <div className={styles.payeeName}>
                    {txn.payeeName || 'Unknown Payee'}
                  </div>
                  <div className={styles.categoryDate}>
                    <span>{txn.categoryName || 'Uncategorized'}</span>
                    <span>•</span>
                    <span>{formatDate(txn.date)}</span>
                  </div>
                </div>
                <div
                  className={`${styles.transactionAmount} ${
                    isInflow ? styles.inflow : styles.outflow
                  }`}
                >
                  {isInflow ? '+' : '-'}{formatCurrency(amount ?? 0)}
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
