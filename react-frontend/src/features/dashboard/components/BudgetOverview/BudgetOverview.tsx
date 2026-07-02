import { useAppSelector } from '@/app/hooks';
import { selectBudgetHealth } from '../../store/dashboardSlice';
import { ChartPie } from '@phosphor-icons/react';
import { formatCurrency } from '../../utils';
import styles from './BudgetOverview.module.css';

export default function BudgetOverview() {
  const budgetHealth = useAppSelector(selectBudgetHealth);

  const topCategories = [...budgetHealth.categories]
    .filter((category) => category.spent > 0)
    .sort((a, b) => b.spent - a.spent)
    .slice(0, 5);
  const maxSpent = topCategories[0]?.spent ?? 0;
  const totalSpent = budgetHealth.categories.reduce((sum, category) => sum + category.spent, 0);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2 className={styles.title}>
          <ChartPie size={18} />
          Top spending
        </h2>
        {totalSpent > 0 && (
          <span className={styles.headerTotal}>{formatCurrency(totalSpent)} total</span>
        )}
      </div>

      {topCategories.length > 0 ? (
        <div className={styles.categoryList}>
          {topCategories.map((category) => (
            <div key={category.id} className={styles.categoryRow}>
              <div className={styles.categoryTopLine}>
                <span className={styles.categoryName}>{category.name}</span>
                <span className={styles.categoryAmount}>{formatCurrency(category.spent)}</span>
              </div>
              <div className={styles.barTrack}>
                <div
                  className={styles.barFill}
                  style={{ width: `${maxSpent > 0 ? (category.spent / maxSpent) * 100 : 0}%` }}
                />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className={styles.emptyState}>No spending this month</div>
      )}
    </div>
  );
}
