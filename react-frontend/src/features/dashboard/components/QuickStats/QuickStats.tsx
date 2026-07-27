import { useAppSelector } from '@/app/hooks';
import { selectDashboardStats, selectMonthlyComparison } from '../../store/dashboardSlice';
import { selectInflowAmount } from '@/features/category/store';
import { selectMonthInHumanFormat } from '@/features/budget/store/budgetSlice';
import { TrendUp, TrendDown } from '@phosphor-icons/react';
import { formatCurrency, formatCompactCurrency } from '../../utils';
import styles from './QuickStats.module.css';

interface DeltaProps {
  delta: number;
  upIsGood: boolean;
}

const DeltaChip = ({ delta, upIsGood }: DeltaProps) => {
  if (delta === 0) return null;
  const isUp = delta > 0;
  const isGood = isUp === upIsGood;

  return (
    <span className={`${styles.delta} ${isGood ? styles.deltaGood : styles.deltaBad}`}>
      {isUp ? <TrendUp size={15} weight="bold" /> : <TrendDown size={15} weight="bold" />}
      {formatCompactCurrency(Math.abs(delta))}
      <span className={styles.deltaContext}>vs last month</span>
    </span>
  );
};

export default function QuickStats() {
  const stats = useAppSelector(selectDashboardStats);
  const comparison = useAppSelector(selectMonthlyComparison);
  const readyToAssign = useAppSelector(selectInflowAmount);
  const monthLabel = useAppSelector(selectMonthInHumanFormat);

  const monthShort = monthLabel ? monthLabel.split(',')[0] : 'This month';

  return (
    <div className={styles.container}>
      <div className={styles.tile}>
        <span className={styles.label}>Net worth</span>
        <span className={styles.value}>
          {stats.netWorth < 0 ? '-' : ''}{formatCurrency(stats.netWorth)}
        </span>
      </div>

      <div className={styles.tile}>
        <span className={styles.label}>Income · {monthShort}</span>
        <span className={styles.value}>{formatCurrency(comparison.income)}</span>
        {comparison.hasPrevious && (
          <DeltaChip delta={comparison.income - comparison.prevIncome} upIsGood />
        )}
      </div>

      <div className={styles.tile}>
        <span className={styles.label}>Spent · {monthShort}</span>
        <span className={styles.value}>{formatCurrency(comparison.expenses)}</span>
        {comparison.hasPrevious && (
          <DeltaChip delta={comparison.expenses - comparison.prevExpenses} upIsGood={false} />
        )}
      </div>

      <div className={styles.tile}>
        <span className={styles.label}>Ready to assign</span>
        <span className={`${styles.value} ${readyToAssign < 0 ? styles.valueNegative : ''}`}>
          {readyToAssign < 0 ? '-' : ''}{formatCurrency(readyToAssign)}
        </span>
      </div>
    </div>
  );
}
