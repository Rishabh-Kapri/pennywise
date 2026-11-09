import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowClockwise, CheckCircle, Warning, WarningCircle } from '@phosphor-icons/react';
import { useHeader } from '../../../context/HeaderContext';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { DateSelector, Skeleton } from '@/components/common';
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { selectUser } from '@/features/auth/store/authSlice';
import { selectSelectedBudget, selectSelectedMonth } from '@/features/budget/store/budgetSlice';
import { fetchInflowAmount, selectInflowAmount, selectInflowLoading } from '@/features/category/store/categorySlice';
import { BalanceCard, CashFlowCard, RecentActivity, SpendingCard } from '@/features/dashboard/components';
import {
  fetchDashboardActivity,
  fetchDashboardCashFlow,
  selectDashboardError,
  selectDashboardReady,
  selectDashboardRevision,
} from '@/features/dashboard/store';
import { formatCurrency } from '@/features/dashboard/utils';
import { GettingStartedChecklist } from '@/features/onboarding/components/GettingStartedChecklist';
import { LoadingState } from '@/utils';
import styles from './Dashboard.module.css';

const getGreeting = (): string => {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 17) return 'Good afternoon';
  return 'Good evening';
};

/** Same wording as the Budget page, so the two never disagree. */
function ReadyToAssign({ amount }: { amount: number }) {
  if (amount < 0) {
    return (
      <Link to="/budget" className={`${styles.ready} ${styles.readyOver}`}>
        <Warning size={15} weight="bold" aria-hidden="true" />
        Assigned <strong>{formatCurrency(Math.abs(amount))}</strong> more than you have
      </Link>
    );
  }
  if (amount === 0) {
    return (
      <Link to="/budget" className={`${styles.ready} ${styles.readyDone}`}>
        <CheckCircle size={15} weight="fill" aria-hidden="true" />
        Every rupee has a job
      </Link>
    );
  }
  return (
    <Link to="/budget" className={styles.ready}>
      <strong>{formatCurrency(amount)}</strong> ready to assign
    </Link>
  );
}

const DashboardSkeleton = () => (
  <div className={styles.grid} aria-hidden="true">
    <Skeleton variant="rectangular" height={420} className={`${styles.skeleton} ${styles.balance}`} />
    <Skeleton variant="rectangular" height={420} className={`${styles.skeleton} ${styles.cashflow}`} />
    <Skeleton variant="rectangular" height={420} className={`${styles.skeleton} ${styles.rail}`} />
    <Skeleton variant="rectangular" height={360} className={`${styles.skeleton} ${styles.spending}`} />
  </div>
);

export default function Dashboard() {
  const { setHeaderContent } = useHeader();
  const dispatch = useAppDispatch();
  const user = useAppSelector(selectUser);
  const budgetId = useAppSelector(selectSelectedBudget)?.id;
  const selectedMonth = useAppSelector(selectSelectedMonth);
  const revision = useAppSelector(selectDashboardRevision);
  const ready = useAppSelector(selectDashboardReady);
  const error = useAppSelector(selectDashboardError);
  const inflowAmount = useAppSelector(selectInflowAmount);
  const inflowLoading = useAppSelector(selectInflowLoading);

  // Keep the pill up through refetches; only the very first load hides it.
  const [inflowLoaded, setInflowLoaded] = useState(inflowLoading === LoadingState.SUCCESS);
  if (!inflowLoaded && inflowLoading === LoadingState.SUCCESS) setInflowLoaded(true);

  // The greeting is the page heading, so the shared header slot stays empty.
  useEffect(() => {
    setHeaderContent(null);
  }, [setHeaderContent]);

  // Balance history walks back from the current balances through the
  // activity, so both reload together whenever a transaction changes.
  useEffect(() => {
    if (!budgetId) return;
    dispatch(fetchDashboardActivity());
    dispatch(fetchAllAccounts());
    dispatch(fetchInflowAmount());
  }, [dispatch, budgetId, revision]);

  useEffect(() => {
    if (!budgetId || !selectedMonth) return;
    dispatch(fetchDashboardCashFlow(selectedMonth));
  }, [dispatch, budgetId, selectedMonth, revision]);

  const retry = () => {
    dispatch(fetchDashboardActivity());
    if (selectedMonth) dispatch(fetchDashboardCashFlow(selectedMonth));
  };

  const firstName = user?.name?.trim().split(/\s+/)[0];
  const today = new Date().toLocaleDateString('en-IN', { weekday: 'long', day: 'numeric', month: 'long' });

  return (
    <div className={styles.container}>
      <header className={styles.top}>
        <div className={styles.hello}>
          <h1 className={styles.greeting}>
            {getGreeting()}
            {firstName && `, ${firstName}`}
          </h1>
          <p className={styles.date}>It's {today}</p>
        </div>
        <div className={styles.topActions}>
          {inflowLoaded && <ReadyToAssign amount={inflowAmount} />}
          <DateSelector monthYearOnly />
        </div>
      </header>

      {error && (
        <div className={styles.error} role="alert">
          <WarningCircle size={18} weight="bold" aria-hidden="true" />
          <span>Some of your numbers didn't load. {error}</span>
          <button type="button" className={styles.retry} onClick={retry}>
            <ArrowClockwise size={14} aria-hidden="true" />
            Retry
          </button>
        </div>
      )}

      <GettingStartedChecklist />

      {!ready ? (
        <DashboardSkeleton />
      ) : (
        <div className={styles.grid}>
          <BalanceCard className={styles.balance} />
          <CashFlowCard className={styles.cashflow} />
          <div className={styles.rail}>
            <RecentActivity />
          </div>
          <SpendingCard className={styles.spending} />
        </div>
      )}
    </div>
  );
}
