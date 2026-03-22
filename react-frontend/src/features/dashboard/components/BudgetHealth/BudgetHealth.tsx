import { useAppSelector } from '@/app/hooks';
import { selectBudgetHealth } from '../../store/dashboardSlice';
import { Activity } from 'lucide-react';
import styles from './BudgetHealth.module.css';

const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(Math.abs(amount));
};

export default function BudgetHealth() {
  const budgetHealth = useAppSelector(selectBudgetHealth);

  // Show categories that need attention (warning or danger first)
  const attentionCategories = [...budgetHealth.categories]
    .filter((c) => c.status === 'danger' || c.status === 'warning')
    .slice(0, 5);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2 className={styles.title}>
          <Activity size={20} className={styles.titleIcon} />
          Budget Health
        </h2>
      </div>

      {/* Summary Cards */}
      <div className={styles.summaryCards}>
        <div className={`${styles.summaryCard} ${styles.healthy}`}>
          <div className={styles.summaryCount}>{budgetHealth.healthyCount}</div>
          <div className={styles.summaryLabel}>On Track</div>
        </div>
        <div className={`${styles.summaryCard} ${styles.warning}`}>
          <div className={styles.summaryCount}>{budgetHealth.warningCount}</div>
          <div className={styles.summaryLabel}>Nearing Limit</div>
        </div>
        <div className={`${styles.summaryCard} ${styles.danger}`}>
          <div className={styles.summaryCount}>{budgetHealth.dangerCount}</div>
          <div className={styles.summaryLabel}>Over Budget</div>
        </div>
      </div>

      {/* Categories Needing Attention */}
      {attentionCategories.length > 0 ? (
        <div className={styles.categoryList}>
          {attentionCategories.map((category) => (
            <div key={category.id} className={styles.categoryItem}>
              <div
                className={`${styles.statusIndicator} ${styles[category.status]}`}
              />
              <span className={styles.categoryName}>{category.name}</span>
              <span
                className={`${styles.categoryRemaining} ${
                  category.remaining >= 0 ? styles.positive : styles.negative
                }`}
              >
                {category.remaining >= 0
                  ? `${formatCurrency(category.remaining)} left`
                  : `${formatCurrency(category.remaining)} over`}
              </span>
            </div>
          ))}
        </div>
      ) : (
        <div className={styles.emptyState}>
          {budgetHealth.categories.length > 0
            ? '✨ All categories are on track!'
            : 'No budget data for this month yet.'}
        </div>
      )}
    </div>
  );
}
