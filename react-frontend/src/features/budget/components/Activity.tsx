import { useAppSelector } from '@/app/hooks';
import { selectCategoryGroups } from '@/features/category/store/categorySlice';
import { selectSelectedMonth } from '../store';
import { getCurrencyLocaleString, getLocaleDate } from '@/utils/date.utils';
import styles from './Activity.module.css';

interface ActivityInfoItemProps {
  title: string;
  amount: string;
}

function ActivityInfoItem({ title, amount }: ActivityInfoItemProps) {
  return (
    <div className={styles.infoItem}>
      <div className={styles.activityItemTitle}>{title}</div>
      <div>{amount}</div>
    </div>
  );
}

export function Activity() {
  const { allCategoryGroups: groups } = useAppSelector(selectCategoryGroups);
  const month = useAppSelector(selectSelectedMonth);
  const localeMonth = getLocaleDate(month, { month: 'long' });
  console.log('Rendering Activity component with groups:', groups, 'and month:', month, 'localeMonth:', localeMonth);

  const totalAssigned = groups.reduce(
    (sum, group) => sum + (group.budgeted?.[month] ?? 0),
    0,
  );

  const totalActivity = groups.reduce(
    (sum, group) => sum + (group.activity?.[month] ?? 0),
    0,
  );

  const totalAvailable = groups.reduce(
    (sum, group) => sum + (group.balance?.[month] ?? 0),
    0,
  );

  return (
    <div className={styles.container}>
      <div className={styles.title}>{localeMonth} Budget</div>
      <div className={styles.budgetInfo}>
        <div className={styles.header}>
          <div>Total Available</div>
          <div>{getCurrencyLocaleString(totalAvailable)}</div>
        </div>
        <hr className={styles.divider} />
        <ActivityInfoItem
          title="Total Assigned"
          amount={getCurrencyLocaleString(totalAssigned)}
        />
        <ActivityInfoItem
          title="Total Activity"
          amount={getCurrencyLocaleString(totalActivity)}
        />
      </div>
    </div>
  );
}
