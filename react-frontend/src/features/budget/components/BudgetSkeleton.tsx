import { Skeleton } from '@/components/common/Skeleton';
import styles from './BudgetSkeleton.module.css';

export function CategoryGroupSkeleton() {
  return (
    <div className={styles.wrapper}>
      <div className={styles.heading}>
        <div className={styles.headingText}>
          <Skeleton width={96} height={14} />
          <Skeleton width={180} height={52} />
          <Skeleton width={420} height={18} />
        </div>
        <Skeleton width={120} height={36} className={styles.countBadge} />
      </div>

      <div className={styles.summaryGrid}>
        {[1, 2, 3].map((i) => (
          <div key={i} className={styles.summaryCard}>
            <Skeleton width={36} height={36} className={styles.summaryIcon} />
            <div className={styles.summaryText}>
              <Skeleton width={72} height={14} />
              <Skeleton width={112} height={22} />
            </div>
          </div>
        ))}
      </div>

      <div className={styles.budgetPanel}>
        <div className={styles.categoryPanel}>
          <div className={styles.tableHeader}>
            <div className={styles.spacer}></div>
            <Skeleton width={80} height={14} />
            <Skeleton width={80} height={14} />
            <Skeleton width={80} height={14} />
          </div>

          <div className={styles.scrollableContent}>
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className={styles.groupContainer}>
                <div className={styles.groupHeader}>
                  <Skeleton variant="circular" width={20} height={20} />
                  <Skeleton width={150} height={20} />
                  <div className={styles.groupAmounts}>
                    <Skeleton width={92} height={18} />
                    <Skeleton width={92} height={18} />
                    <Skeleton width={92} height={24} />
                  </div>
                </div>

                {[1, 2, 3].map((j) => (
                  <div key={j} className={styles.categoryItem}>
                    <Skeleton width="38%" height={18} />
                    <Skeleton width={92} height={18} />
                    <Skeleton width={92} height={18} />
                    <Skeleton width={92} height={24} />
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>

        <aside className={styles.detailPanel}>
          <Skeleton width={160} height={24} />
          <div className={styles.detailCard}>
            <Skeleton width="100%" height={20} />
            <Skeleton width="70%" height={16} />
            <Skeleton width="82%" height={16} />
            <Skeleton width="64%" height={16} />
          </div>
          <div className={styles.detailCard}>
            <Skeleton width={80} height={18} />
            <Skeleton width="100%" height={120} />
          </div>
        </aside>
      </div>
    </div>
  );
}

export function BudgetHeaderSkeleton() {
  return (
    <div className={styles.headerContainer}>
      <Skeleton width={200} height={40} />
      <Skeleton width={180} height={56} />
    </div>
  );
}
