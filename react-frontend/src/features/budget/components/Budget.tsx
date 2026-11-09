import { useEffect, useMemo } from 'react';
import { useHeader } from '../../../context/HeaderContext';
import { DateSelector, Skeleton } from '@/components/common';
import styles from './Budget.module.css';
import { CheckCircle } from '@phosphor-icons/react';
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
import { BudgetTableSkeleton } from './BudgetSkeleton';
import { formatBudgetAmount } from '@/features/category/components/budgetFormat';

function readyToAssignCopy(amount: number) {
  if (amount < 0) return { label: 'Assigned more than you have', tone: styles.toneOver };
  if (amount === 0) return { label: 'Every rupee has a job', tone: styles.toneDone };
  return { label: 'Ready to assign', tone: styles.toneReady };
}

export default function Budget() {
  const { setHeaderContent } = useHeader();

  const { allCategoryGroups: groups } = useAppSelector(selectCategoryGroups);
  const inflowAmount = useAppSelector(selectInflowAmount);
  const month = useAppSelector(selectSelectedMonth);
  const categoryGroupsLoading = useAppSelector(selectCategoryLoading);
  const inflowLoading = useAppSelector(selectInflowLoading);

  const totals = useMemo(
    () =>
      groups.reduce(
        (sum, group) => ({
          assigned: sum.assigned + (group.budgeted?.[month] ?? 0),
          activity: sum.activity + (group.activity?.[month] ?? 0),
          available: sum.available + (group.balance?.[month] ?? 0),
        }),
        { assigned: 0, activity: 0, available: 0 },
      ),
    [groups, month],
  );

  // The month picker lives in the page heading, so the shared header slot stays empty.
  useEffect(() => {
    setHeaderContent(null);
  }, [setHeaderContent]);

  const ready = readyToAssignCopy(inflowAmount);
  const isLoading = categoryGroupsLoading === LoadingState.PENDING;

  return (
    <section className={styles.page}>
      <header className={styles.top}>
        <h1>Budget</h1>
        <DateSelector monthYearOnly />
      </header>

      <div className={styles.overview}>
        <div className={`${styles.ready} ${ready.tone}`}>
          <span className={styles.readyLabel}>
            {inflowAmount === 0 && <CheckCircle size={15} weight="fill" />}
            {ready.label}
          </span>
          {inflowLoading === LoadingState.PENDING ? (
            <Skeleton width={180} height={40} />
          ) : (
            <strong className={styles.readyAmount}>{formatBudgetAmount(inflowAmount)}</strong>
          )}
        </div>

        <dl className={styles.stats}>
          <div>
            <dt>Assigned</dt>
            <dd>{formatBudgetAmount(totals.assigned)}</dd>
          </div>
          <div>
            <dt>Activity</dt>
            <dd>{formatBudgetAmount(totals.activity)}</dd>
          </div>
          <div>
            <dt>Available</dt>
            <dd className={totals.available < 0 ? styles.negative : undefined}>
              {formatBudgetAmount(totals.available)}
            </dd>
          </div>
        </dl>
      </div>

      <div className={styles.content}>
        {isLoading ? <BudgetTableSkeleton /> : <CategoryGroup groups={groups} month={month} />}
      </div>
    </section>
  );
}
