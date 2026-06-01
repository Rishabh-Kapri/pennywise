import { Skeleton } from '@/components/common';
import styles from './TransactionSkeleton.module.css';

const GROUPS = [4, 5, 3];

function SkeletonRow({ index, total }: { index: number; total: number }) {
  const cardClass =
    total === 1
      ? styles.cardSingle
      : index === 0
        ? styles.cardFirst
        : index === total - 1
          ? styles.cardLast
          : '';

  return (
    <div className={`${styles.rowWrapper} ${cardClass} ${index < total - 1 ? styles.rowDivider : ''}`}>
      <div className={styles.row}>
        <div className={styles.dateCell}>
          <Skeleton width="5.5rem" height={18} />
        </div>
        <div className={styles.payeeCell}>
          <Skeleton width="70%" height={24} />
        </div>
        <div className={styles.categoryCell}>
          <Skeleton width="8rem" height={30} />
        </div>
        <div className={styles.noteCell}>
          <Skeleton width="62%" height={18} />
        </div>
        <div className={styles.amountCell}>
          <Skeleton width="5rem" height={20} />
        </div>
      </div>
    </div>
  );
}

export function TransactionSkeleton() {
  return (
    <div className={styles.wrapper}>
      <div className={styles.headerRow}>
        <Skeleton width="5rem" height={14} />
        <Skeleton width="6rem" height={14} />
        <Skeleton width="6rem" height={14} />
        <Skeleton width="4rem" height={14} />
        <Skeleton width="5rem" height={14} />
      </div>
      <div className={styles.content}>
        {GROUPS.map((rowCount, groupIndex) => (
          <div key={groupIndex} className={styles.group}>
            <div className={styles.monthHeader}>
              <Skeleton width="7rem" height={18} />
              <Skeleton width="12rem" height={16} />
            </div>
            <div className={styles.cardGroup}>
              {Array.from({ length: rowCount }, (_, rowIndex) => (
                <SkeletonRow key={rowIndex} index={rowIndex} total={rowCount} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
