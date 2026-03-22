import { useEffect } from 'react';
import { useHeader } from '../../../context/HeaderContext';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { fetchAllTransaction } from '@/features/transactions/store';
import { fetchAllCategoryGroups, fetchInflowAmount } from '@/features/category/store';
import { selectSelectedMonth } from '@/features/budget/store/budgetSlice';
import { selectDashboardLoading } from '@/features/dashboard/store';
import {
  QuickStats,
  BudgetOverview,
  SpendingTrends,
  RecentTransactions,
  BudgetHealth,
} from '@/features/dashboard/components';
import { LoadingState } from '@/utils';
import styles from './Dashboard.module.css';

const DashboardHeaderContent = () => (
  <div>
    <div style={{ fontSize: '1rem', fontWeight: 600, color: 'var(--color-text)' }}>
      Dashboard
    </div>
  </div>
);

const getGreeting = (): string => {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 17) return 'Good afternoon';
  return 'Good evening';
};

const LoadingSkeleton = () => (
  <div className={styles.loadingContainer}>
    <div className={`${styles.skeleton} ${styles.skeletonStats}`} />
    <div className={styles.gridLayout}>
      <div className={`${styles.skeleton} ${styles.skeletonWidget}`} />
      <div className={`${styles.skeleton} ${styles.skeletonWidget}`} />
      <div className={`${styles.skeleton} ${styles.skeletonWidget}`} />
      <div className={`${styles.skeleton} ${styles.skeletonWidget}`} />
    </div>
  </div>
);

export default function Dashboard() {
  const { setHeaderContent } = useHeader();
  const dispatch = useAppDispatch();
  const selectedMonth = useAppSelector(selectSelectedMonth);
  const loading = useAppSelector(selectDashboardLoading);

  useEffect(() => {
    setHeaderContent(<DashboardHeaderContent />);
    return () => setHeaderContent(null);
  }, [setHeaderContent]);

  // Fetch all required data for the dashboard
  useEffect(() => {
    dispatch(fetchAllAccounts());
    dispatch(fetchAllTransaction(''));
    if (selectedMonth) {
      dispatch(fetchAllCategoryGroups(selectedMonth));
      dispatch(fetchInflowAmount());
    }
  }, [dispatch, selectedMonth]);

  if (loading === LoadingState.PENDING) {
    return (
      <div className={styles.container}>
        <LoadingSkeleton />
      </div>
    );
  }

  return (
    <div className={styles.container}>
      {/* Greeting */}
      <div className={styles.greeting}>
        <h1 className={styles.greetingTitle}>{getGreeting()}, Rishabh!</h1>
        <p className={styles.greetingSubtitle}>
          Here's your financial overview for this month
        </p>
      </div>

      {/* Quick Stats */}
      <section className={styles.statsSection}>
        <QuickStats />
      </section>

      {/* Two-column Grid Layout */}
      <div className={styles.gridLayout}>
        {/* Budget Overview */}
        <BudgetOverview />

        {/* Budget Health */}
        <BudgetHealth />

        {/* Spending Trends - Full Width */}
        <div className={styles.fullWidth}>
          <SpendingTrends />
        </div>

        {/* Recent Transactions - Full Width */}
        <div className={styles.fullWidth}>
          <RecentTransactions />
        </div>
      </div>
    </div>
  );
}
