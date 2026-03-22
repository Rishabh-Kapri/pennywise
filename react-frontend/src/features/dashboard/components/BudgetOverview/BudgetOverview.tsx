import { useAppSelector } from '@/app/hooks';
import { selectBudgetHealth } from '../../store/dashboardSlice';
import { selectInflowAmount } from '@/features/category/store';
import { PieChart, CheckCircle } from 'lucide-react';
import styles from './BudgetOverview.module.css';

const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount);
};

export default function BudgetOverview() {
  const inflowAmount = useAppSelector(selectInflowAmount);
  const budgetHealth = useAppSelector(selectBudgetHealth);

  // Show top 5 categories by budget percentage used
  const topCategories = [...budgetHealth.categories]
    .filter((c) => c.budgeted > 0)
    .sort((a, b) => b.percentUsed - a.percentUsed)
    .slice(0, 5);

  const getReadyToAssignClass = () => {
    if (inflowAmount > 0) return styles.positive;
    if (inflowAmount < 0) return styles.negative;
    return styles.zero;
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2 className={styles.title}>
          <PieChart size={20} className={styles.titleIcon} />
          Budget Overview
        </h2>
        <div className={`${styles.readyToAssign} ${getReadyToAssignClass()}`}>
          {inflowAmount === 0 ? (
            <>
              <CheckCircle size={16} />
              All Assigned
            </>
          ) : (
            <>{formatCurrency(inflowAmount)} to assign</>
          )}
        </div>
      </div>

      {topCategories.length > 0 ? (
        <div className={styles.categoryList}>
          {topCategories.map((category) => (
            <div key={category.id} className={styles.categoryItem}>
              <div className={styles.categoryHeader}>
                <span className={styles.categoryName}>{category.name}</span>
                <span className={styles.categoryValues}>
                  {formatCurrency(category.spent)} / {formatCurrency(category.budgeted)}
                </span>
              </div>
              <div className={styles.progressBar}>
                <div
                  className={`${styles.progressFill} ${styles[category.status]}`}
                  style={{ width: `${Math.min(category.percentUsed, 100)}%` }}
                />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className={styles.emptyState}>
          No budget data for this month yet.
        </div>
      )}
    </div>
  );
}
