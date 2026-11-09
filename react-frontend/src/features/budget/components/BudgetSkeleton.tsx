import { Skeleton } from '@/components/common/Skeleton';
import styles from './BudgetSkeleton.module.css';

const ROWS_PER_GROUP = [3, 2, 3, 2];

export function BudgetTableSkeleton() {
  return (
    <div className={styles.table} aria-busy="true" aria-label="Loading budget">
      <div className={styles.columns}>
        <Skeleton width={64} height={10} />
        <Skeleton width={56} height={10} />
        <Skeleton width={56} height={10} />
        <Skeleton width={60} height={10} />
      </div>
      {ROWS_PER_GROUP.map((rows, groupIndex) => (
        <div key={groupIndex} className={styles.group}>
          <div className={styles.groupRow}>
            <Skeleton width={120} height={14} />
            <Skeleton width={64} height={12} />
            <Skeleton width={64} height={12} />
            <Skeleton width={64} height={12} />
          </div>
          {Array.from({ length: rows }, (_, rowIndex) => (
            <div key={rowIndex} className={styles.row}>
              <div className={styles.name}>
                <Skeleton width={`${40 + ((groupIndex + rowIndex) % 3) * 12}%`} height={14} />
                <Skeleton width="45%" height={3} />
              </div>
              <Skeleton width={56} height={14} />
              <Skeleton width={56} height={14} />
              <Skeleton width={64} height={22} />
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}
