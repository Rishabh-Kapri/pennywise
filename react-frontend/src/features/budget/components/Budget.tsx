import { useEffect, useMemo } from 'react';
import { useHeader } from '../../../context/HeaderContext';
import { DateSelector, Skeleton } from '@/components/common';
import styles from './Budget.module.css';
import { Check, CurrencyCircleDollar as CircleDollarSign, Stack as Layers3, Wallet as WalletCards } from '@phosphor-icons/react';
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

  const budgetSummary = useMemo(() => {
    return groups.reduce(
      (summary, group) => {
        summary.assigned += group.budgeted?.[month] ?? 0;
        summary.activity += group.activity?.[month] ?? 0;
        summary.available += group.balance?.[month] ?? 0;
        summary.categories += group.categories?.length ?? 0;
        return summary;
      },
      { assigned: 0, activity: 0, available: 0, categories: 0 },
    );
  }, [groups, month]);

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
    return (
      <section className={styles.page}>
        <CategoryGroupSkeleton />
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <div className={styles.heading}>
        <div>
          <span className={styles.kicker}>Monthly plan</span>
          <h1>Budget</h1>
          <p>Assign your inflow, track category activity, and keep available money in view.</p>
        </div>
        <div className={styles.countBadge}>{budgetSummary.categories} categories</div>
      </div>

      <div className={styles.summaryGrid}>
        <div className={styles.summaryCard}>
          <span className={styles.summaryIcon}><CircleDollarSign size={18} /></span>
          <span>Assigned</span>
          <strong>{getCurrencyLocaleString(budgetSummary.assigned)}</strong>
        </div>
        <div className={styles.summaryCard}>
          <span className={styles.summaryIcon}><WalletCards size={18} /></span>
          <span>Activity</span>
          <strong>{getCurrencyLocaleString(budgetSummary.activity)}</strong>
        </div>
        <div className={styles.summaryCard}>
          <span className={styles.summaryIcon}><Layers3 size={18} /></span>
          <span>Available</span>
          <strong>{getCurrencyLocaleString(budgetSummary.available)}</strong>
        </div>
      </div>

      <div className={styles.budgetPanel}>
        <CategoryGroup groups={groups} month={month} />
      </div>
    </section>
  );
}
