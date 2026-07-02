import { useAppSelector } from '@/app/hooks';
import { selectBudgetHealth } from '../../store/dashboardSlice';
import { Heartbeat } from '@phosphor-icons/react';
import type { CategoryHealth } from '../../types';
import { formatCurrency } from '../../utils';
import styles from './BudgetHealth.module.css';

const STATUS_ORDER: Record<CategoryHealth['status'], number> = {
  danger: 0,
  warning: 1,
  healthy: 2,
};

const statusClass = (status: CategoryHealth['status']) => {
  if (status === 'danger') return styles.danger;
  if (status === 'warning') return styles.warning;
  return styles.healthy;
};

export default function BudgetHealth() {
  const budgetHealth = useAppSelector(selectBudgetHealth);

  const categories = [...budgetHealth.categories]
    .sort((a, b) =>
      STATUS_ORDER[a.status] !== STATUS_ORDER[b.status]
        ? STATUS_ORDER[a.status] - STATUS_ORDER[b.status]
        : b.percentUsed - a.percentUsed,
    )
    .slice(0, 6);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2 className={styles.title}>
          <Heartbeat size={18} />
          Budget health
        </h2>
      </div>

      <div className={styles.summaryRow}>
        <span className={`${styles.summaryChip} ${styles.healthy}`}>
          <span className={styles.summaryDot} />
          {budgetHealth.healthyCount} healthy
        </span>
        <span className={`${styles.summaryChip} ${styles.warning}`}>
          <span className={styles.summaryDot} />
          {budgetHealth.warningCount} near limit
        </span>
        <span className={`${styles.summaryChip} ${styles.danger}`}>
          <span className={styles.summaryDot} />
          {budgetHealth.dangerCount} over
        </span>
      </div>

      {categories.length > 0 ? (
        <div className={styles.categoryList}>
          {categories.map((category) => (
            <div key={category.id} className={styles.categoryRow}>
              <div className={styles.categoryTopLine}>
                <span className={styles.categoryName}>{category.name}</span>
                <span
                  className={`${styles.categoryAmount} ${category.remaining < 0 ? styles.amountOver : ''}`}
                >
                  {formatCurrency(category.remaining)} {category.remaining < 0 ? 'over' : 'left'}
                </span>
              </div>
              <div className={`${styles.meterTrack} ${statusClass(category.status)}`}>
                <div
                  className={styles.meterFill}
                  style={{ width: `${Math.max(category.percentUsed, 2)}%` }}
                />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className={styles.emptyState}>No budgeted categories this month</div>
      )}
    </div>
  );
}
