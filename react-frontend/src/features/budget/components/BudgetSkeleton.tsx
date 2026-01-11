import { Skeleton } from '@/components/common/Skeleton';
import styles from './BudgetSkeleton.module.css';

export function CategoryGroupSkeleton() {
  return (
    <div className={styles.wrapper}>
      {/* Table Header Skeleton */}
      <div className={styles.tableHeader}>
        <div className={styles.spacer}></div>
        <Skeleton width={80} height={16} />
        <Skeleton width={80} height={16} />
        <Skeleton width={80} height={16} />
      </div>

      {/* Category Groups Skeleton */}
      <div className={styles.scrollableContent}>
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className={styles.groupContainer}>
            {/* Group Header */}
            <div className={styles.groupHeader}>
              <Skeleton variant="circular" width={20} height={20} />
              <Skeleton width={150} height={20} />
            </div>

            {/* Category Items */}
            {[1, 2, 3].map((j) => (
              <div key={j} className={styles.categoryItem}>
                <Skeleton width={475} height={18} />
                <Skeleton width={100} height={18} />
                <Skeleton width={100} height={18} />
                <Skeleton width={100} height={18} />
              </div>
            ))}
          </div>
        ))}
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
