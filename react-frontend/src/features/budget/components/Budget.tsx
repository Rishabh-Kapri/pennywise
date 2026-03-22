import { useEffect } from 'react';
import { useHeader } from '../../../context/HeaderContext';
import { DateSelector, Skeleton } from '@/components/common';
import styles from './Budget.module.css';
import { Check } from 'lucide-react';
import { CategoryGroup } from '@/features/category';
import { useAppSelector } from '@/app/hooks';
import {
  selectCategoryGroups,
  selectCategoryLoading,
  selectInflowAmount,
  selectInflowLoading,
} from '@/features/category/store/categorySlice';
import { selectSelectedMonth } from '../store';
import { LoadingState } from '@/utils';
import { CategoryGroupSkeleton } from './BudgetSkeleton';
import { getCurrencyLocaleString } from '@/utils/date.utils';

interface BudgetProps {
  inflowAmount: number;
  inflowLoading: LoadingState;
}

const BudgetHeaderContent = ({ inflowAmount, inflowLoading }: BudgetProps) => (
  <div className={styles.container}>
    <DateSelector monthYearOnly />
    {inflowLoading === LoadingState.PENDING && <div>
      <Skeleton width={130} height={64} />
    </div>}
    {inflowLoading === LoadingState.SUCCESS && (
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
    )}
  </div>
);

export default function Budget() {
  const { setHeaderContent } = useHeader();

  const { allCategoryGroups: groups } = useAppSelector(selectCategoryGroups);
  const inflowAmount = useAppSelector(selectInflowAmount);
  const month = useAppSelector(selectSelectedMonth);
  const categoryGroupsLoading = useAppSelector(selectCategoryLoading);
  const inflowLoading = useAppSelector(selectInflowLoading);

  useEffect(() => {
    setHeaderContent(
      <BudgetHeaderContent
        inflowAmount={inflowAmount}
        inflowLoading={inflowLoading}
      />,
    );

    // clear header content when component unmounts
    return () => setHeaderContent(null);
  }, [setHeaderContent, inflowAmount, inflowLoading]);

  if (categoryGroupsLoading === LoadingState.PENDING) {
    return <CategoryGroupSkeleton />;
  }

  return <CategoryGroup groups={groups} month={month} />;
}
