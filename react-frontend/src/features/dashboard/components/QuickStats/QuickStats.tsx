import { useAppSelector } from '@/app/hooks';
import { selectDashboardStats } from '../../store/dashboardSlice';
import { TrendingUp, TrendingDown, Wallet } from 'lucide-react';
import styles from './QuickStats.module.css';

const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount);
};

export default function QuickStats() {
  const stats = useAppSelector(selectDashboardStats);

  return (
    <div className={styles.container}>
      {/* Total Income */}
      <div className={`${styles.statCard} ${styles.income}`}>
        <div className={styles.header}>
          <div className={`${styles.icon} ${styles.income}`}>
            <TrendingUp size={20} />
          </div>
          <span className={styles.label}>Total Income</span>
        </div>
        <div className={`${styles.value} ${styles.positive}`}>
          {formatCurrency(stats.totalIncome)}
        </div>
      </div>

      {/* Total Expenses */}
      <div className={`${styles.statCard} ${styles.expenses}`}>
        <div className={styles.header}>
          <div className={`${styles.icon} ${styles.expenses}`}>
            <TrendingDown size={20} />
          </div>
          <span className={styles.label}>Total Expenses</span>
        </div>
        <div className={`${styles.value} ${styles.negative}`}>
          {formatCurrency(stats.totalExpenses)}
        </div>
      </div>

      {/* Net Worth */}
      <div className={`${styles.statCard} ${styles.netWorth}`}>
        <div className={styles.header}>
          <div className={`${styles.icon} ${styles.netWorth}`}>
            <Wallet size={20} />
          </div>
          <span className={styles.label}>Net Worth</span>
        </div>
        <div className={`${styles.value} ${stats.netWorth >= 0 ? styles.positive : styles.negative}`}>
          {formatCurrency(stats.netWorth)}
        </div>
      </div>
    </div>
  );
}
