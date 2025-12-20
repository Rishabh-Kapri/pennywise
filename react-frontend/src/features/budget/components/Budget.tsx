import { useEffect } from 'react';
import { useHeader } from '../../../context/HeaderContext';
import { DateSelector } from '@/components/common';
import styles from './Budget.module.css';
import { Check } from 'lucide-react';
import { CategoryGroup } from '@/features/category';
import { useAppSelector } from '@/app/hooks';
import { selectCategoryGroups } from '@/features/category/store/categoryGroupSlice';
import { selectSelectedMonth } from '../store';
import { LoadingState } from '@/utils';
import { CategoryGroupSkeleton } from './BudgetSkeleton';
import { getCurrencyLocaleString } from '@/utils/date.utils';

interface BudgetProps {
  inflowAmount: number;
}

const BudgetHeaderContent = ({ inflowAmount }: BudgetProps) => (
  <div className={styles.container}>
    <DateSelector />
    <div
      className={
        inflowAmount === 0
          ? `${styles.amountContainer} ${styles.allAssigned}`
          : `${styles.amountContainer}`
      }>
      <div className={styles.assignAmount}>
        <span className={styles.title}>
          {getCurrencyLocaleString(inflowAmount)}
        </span>
        <span className={styles.subtitle}>
          {inflowAmount === 0 ? 'All assigned' : 'Ready to assign'}
        </span>
      </div>
      {inflowAmount === 0 && (
        <Check size="1.5rem" className={styles.allAssignedIcon} />
      )}
    </div>
  </div>
);

export default function Budget() {
  const { setHeaderContent } = useHeader();

  const { allCategoryGroups: groups } = useAppSelector(selectCategoryGroups);
  const inflowAmount = useAppSelector((state) => state.categoryGroups.inflow);
  const month = useAppSelector(selectSelectedMonth);
  const categoryGroupsLoading = useAppSelector(
    (state) => state.categoryGroups.loading,
  );

  useEffect(() => {
    setHeaderContent(<BudgetHeaderContent inflowAmount={inflowAmount} />);

    // clear header content when component unmounts
    return () => setHeaderContent(null);
  }, [setHeaderContent, inflowAmount]);

  if (categoryGroupsLoading === LoadingState.PENDING) {
    return <CategoryGroupSkeleton />;
  }

  return <CategoryGroup groups={groups} month={month} />;
}
