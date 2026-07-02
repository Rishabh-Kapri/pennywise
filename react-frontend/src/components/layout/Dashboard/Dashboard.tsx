import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useHeader } from '../../../context/HeaderContext';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { TrackingAccountType } from '@/features/accounts/types/account.types';
import { fetchAllTransaction } from '@/features/transactions/store';
import { fetchAllCategoryGroups, fetchInflowAmount } from '@/features/category/store';
import { selectMonthInHumanFormat, selectSelectedMonth } from '@/features/budget/store/budgetSlice';
import { selectDashboardLoading } from '@/features/dashboard/store';
import {
  QuickStats,
  SpendingTrends,
  BudgetHealth,
  BudgetOverview,
  RecentTransactions,
} from '@/features/dashboard';
import { formatCurrency } from '@/features/dashboard/utils';
import { Bank } from '@phosphor-icons/react';
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

/** First day of the month five months back — the 6-month trend window. */
const getTrendStartDate = (): string => {
  const now = new Date();
  const start = new Date(now.getFullYear(), now.getMonth() - 5, 1);
  const month = String(start.getMonth() + 1).padStart(2, '0');
  return `${start.getFullYear()}-${month}-01`;
};

const LoadingSkeleton = () => (
  <div className={styles.loadingContainer}>
    <div className={styles.skeletonStatsRow}>
      <div className={styles.skeleton} />
      <div className={styles.skeleton} />
      <div className={styles.skeleton} />
      <div className={styles.skeleton} />
    </div>
    <div className={styles.skeletonGrid}>
      <div className={`${styles.skeleton} ${styles.skeletonWidget}`} />
      <div className={`${styles.skeleton} ${styles.skeletonWidget}`} />
    </div>
  </div>
);

export default function Dashboard() {
  const { setHeaderContent } = useHeader();
  const dispatch = useAppDispatch();
  const selectedMonth = useAppSelector(selectSelectedMonth);
  const selectedMonthLabel = useAppSelector(selectMonthInHumanFormat);
  const loading = useAppSelector(selectDashboardLoading);
  const accounts = useAppSelector((state) => [
    ...state.accounts.budgetAccounts,
    ...state.accounts.trackingAccounts.filter(
      (account) => account.type === TrackingAccountType.ASSET,
    ),
  ]);

  const availableCash = accounts
    .filter((account) => (account.balance ?? 0) >= 0)
    .reduce((sum, account) => sum + (account.balance ?? 0), 0);
  const totalDebt = Math.abs(
    accounts
      .filter((account) => (account.balance ?? 0) < 0)
      .reduce((sum, account) => sum + (account.balance ?? 0), 0),
  );

  useEffect(() => {
    setHeaderContent(<DashboardHeaderContent />);
    return () => setHeaderContent(null);
  }, [setHeaderContent]);

  useEffect(() => {
    dispatch(fetchAllAccounts());
    dispatch(fetchAllTransaction({ startDate: getTrendStartDate(), limit: 1000 }));
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
      <div className={styles.pageHeader}>
        <div>
          <h1 className={styles.greeting}>{getGreeting()}, RK</h1>
          <p className={styles.dateSubtitle}>
            {new Date().toLocaleDateString('en-IN', {
              weekday: 'long',
              day: 'numeric',
              month: 'long',
            })}
          </p>
        </div>
        <span className={styles.monthPill}>{selectedMonthLabel || 'This month'}</span>
      </div>

      <QuickStats />

      <div className={styles.dashboardShell}>
        <main className={styles.mainColumn}>
          <SpendingTrends />

          <div className={styles.widgetRow}>
            <BudgetHealth />
            <BudgetOverview />
          </div>

          <section className={styles.accountsCard}>
            <div className={styles.accountsHeader}>
              <h2 className={styles.accountsTitle}>
                <Bank size={18} />
                Accounts
              </h2>
              <div className={styles.accountsStats}>
                <span>
                  Cash <strong>{formatCurrency(availableCash)}</strong>
                </span>
                <span>
                  Debt <strong className={totalDebt > 0 ? styles.debtValue : ''}>
                    {formatCurrency(totalDebt)}
                  </strong>
                </span>
              </div>
            </div>

            {accounts.length > 0 ? (
              <div className={styles.accountList}>
                {accounts.map((account) => (
                  <Link
                    key={account.id ?? account.name}
                    to={account.id ? `/transactions/${account.id}` : '/transactions'}
                    className={styles.accountPill}
                  >
                    <span className={styles.accountDot}>
                      {account.name.charAt(0).toUpperCase()}
                    </span>
                    <span className={styles.accountName}>{account.name}</span>
                    <span
                      className={`${styles.accountBalance} ${(account.balance ?? 0) < 0 ? styles.debtValue : ''}`}
                    >
                      {(account.balance ?? 0) < 0 ? '-' : ''}
                      {formatCurrency(account.balance ?? 0)}
                    </span>
                  </Link>
                ))}
              </div>
            ) : (
              <div className={styles.emptyInline}>No accounts yet</div>
            )}
          </section>
        </main>

        <aside className={styles.recentRail}>
          <RecentTransactions />
        </aside>
      </div>
    </div>
  );
}
