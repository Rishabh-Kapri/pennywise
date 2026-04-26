import { Skeleton } from '@/components/common';
import styles from './TransactionSkeleton.module.css';

export function TransactionSkeleton() {
  return (
    <div className={styles.wrapper}>
      <div className={styles.content}>
        {[...Array(13).keys()].map((i) => (
          <div key={i} className={styles.rowContainer}>
            <div className={styles.row}>
              <Skeleton width={'100%'} height={55} />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
