import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useHeader } from '../../../context/HeaderContext';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { TrackingAccountType } from '@/features/accounts/types/account.types';
import { fetchAllTransaction } from '@/features/transactions/store';
import { fetchAllCategoryGroups, fetchInflowAmount } from '@/features/category/store';
import { selectMonthInHumanFormat, selectSelectedMonth } from '@/features/budget/store/budgetSlice';
import {
  selectBudgetHealth,
  selectDashboardLoading,
  selectRecentTransactions,
} from '@/features/dashboard/store';
import {
  ArrowDownLeft,
  ArrowUpRight,
  Landmark,
  PieChart,
  WalletCards,
} from 'lucide-react';
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

const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(Math.abs(amount));
};

const formatShortDate = (dateStr: string): string => {
  const date = new Date(dateStr);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);

  const time = date.toLocaleTimeString('en-IN', {
    hour: 'numeric',
    minute: '2-digit',
  });

  if (date.toDateString() === today.toDateString()) return `Today, ${time}`;
  if (date.toDateString() === yesterday.toDateString()) return `Yesterday, ${time}`;

  return date.toLocaleDateString('en-IN', {
    month: 'short',
    day: 'numeric',
  });
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
  const selectedMonthLabel = useAppSelector(selectMonthInHumanFormat);
  const loading = useAppSelector(selectDashboardLoading);
  const accounts = useAppSelector((state) => [
    ...state.accounts.budgetAccounts,
    ...state.accounts.trackingAccounts.filter(
      (account) => account.type === TrackingAccountType.ASSET,
    ),
  ]);
  const transactions = useAppSelector((state) => state.transactions.transactions);
  const budgetHealth = useAppSelector(selectBudgetHealth);
  const recentTransactions = useAppSelector(selectRecentTransactions);

  const totalBalance = accounts.reduce((sum, account) => sum + (account.balance ?? 0), 0);
  const positiveAccounts = accounts.filter((account) => (account.balance ?? 0) >= 0);
  const debtAccounts = accounts.filter((account) => (account.balance ?? 0) < 0);
  const availableCash = positiveAccounts.reduce((sum, account) => sum + (account.balance ?? 0), 0);
  const totalDebt = Math.abs(debtAccounts.reduce((sum, account) => sum + (account.balance ?? 0), 0));
  const totalInflow = transactions.reduce((sum, txn) => sum + (txn.inflow ?? 0), 0);
  const totalOutflow = transactions.reduce((sum, txn) => sum + (txn.outflow ?? 0), 0);
  const leftToBudget = totalInflow - totalOutflow;
  const overspentCategories = [...budgetHealth.categories]
    .filter((category) => category.remaining < 0)
    .sort((a, b) => a.remaining - b.remaining)
    .slice(0, 4);
  const spendingCategories = [...budgetHealth.categories]
    .filter((category) => category.spent > 0)
    .sort((a, b) => b.spent - a.spent)
    .slice(0, 5);
  useEffect(() => {
    setHeaderContent(<DashboardHeaderContent />);
    return () => setHeaderContent(null);
  }, [setHeaderContent]);

  useEffect(() => {
    dispatch(fetchAllAccounts());
    dispatch(fetchAllTransaction());
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
          <p className={styles.eyebrow}>{getGreeting()}, RK</p>
          <h1 className={styles.dateTitle}>
            {new Date().toLocaleDateString('en-IN', {
              day: 'numeric',
              month: 'long',
              weekday: 'long',
            })}
          </h1>
        </div>
      </div>

      <div className={styles.dashboardShell}>
        <main className={styles.mainColumn}>
          <section className={styles.contentGrid}>
            <article className={`${styles.card} ${styles.accountsCard}`}>
              <div className={styles.cardTitleRow}>
                <div className={styles.cardTitle}>
                  <Landmark size={18} />
                  Accounts
                </div>
                <div className={styles.cardTitle}>
                  <WalletCards size={18} />
                  Total Cash
                </div>
              </div>

              <div className={styles.accountsOverview}>
                <div className={styles.accountMetricGrid}>
                  <div className={styles.accountMetric}>
                    <span>Total balance</span>
                    <strong>{totalBalance < 0 ? '-' : ''}{formatCurrency(totalBalance)}</strong>
                  </div>
                  <div className={styles.accountMetric}>
                    <span>Available cash</span>
                    <strong>{formatCurrency(availableCash)}</strong>
                  </div>
                  <div className={styles.accountMetric}>
                    <span>Debt</span>
                    <strong>{formatCurrency(totalDebt)}</strong>
                  </div>
                  <div className={styles.accountMetric}>
                    <span>Open accounts</span>
                    <strong>{accounts.length}</strong>
                  </div>
                </div>

                <div className={styles.accountList}>
                  {accounts.map((account) => (
                    <Link
                      key={account.id ?? account.name}
                      to={account.id ? `/transactions/${account.id}` : '/transactions'}
                      className={styles.accountPill}
                    >
                      <span className={styles.accountDot}>{account.name.charAt(0).toUpperCase()}</span>
                      <span className={styles.accountName}>{account.name}</span>
                      <strong>{account.balance && account.balance < 0 ? '-' : ''}{formatCurrency(account.balance ?? 0)}</strong>
                    </Link>
                  ))}
                  {accounts.length === 0 && (
                    <div className={styles.emptyInline}>No accounts yet</div>
                  )}
                </div>
              </div>
            </article>

            <article className={`${styles.card} ${styles.budgetCard}`}>
              <div className={styles.cardTitleRow}>
                <div className={styles.cardTitle}>
                  <PieChart size={18} />
                  Budget Overview
                </div>
                <span className={styles.monthPill}>{selectedMonthLabel || 'This month'}</span>
              </div>
              <div className={styles.budgetSummaryGrid}>
                <div>
                  <span>Incoming</span>
                  <strong>+{formatCurrency(totalInflow)}</strong>
                </div>
                <div>
                  <span>Outgoing</span>
                  <strong>-{formatCurrency(totalOutflow)}</strong>
                </div>
                <div>
                  <span>Left</span>
                  <strong>{leftToBudget < 0 ? '-' : ''}{formatCurrency(leftToBudget)}</strong>
                </div>
              </div>

              <div className={styles.budgetCategoryPanel}>
                <div className={styles.panelHeading}>Overspending</div>
                {overspentCategories.length > 0 ? (
                  overspentCategories.map((category) => (
                    <div key={category.id} className={`${styles.budgetCategoryRow} ${styles.overspentRow}`}>
                      <div>
                        <span>{category.name}</span>
                        <small>{formatCurrency(category.spent)} spent of {formatCurrency(category.budgeted)}</small>
                      </div>
                      <strong>-{formatCurrency(category.remaining)}</strong>
                    </div>
                  ))
                ) : (
                  <div className={styles.emptyInline}>No overspending</div>
                )}

                <div className={styles.panelHeading}>Top spent categories</div>
                {spendingCategories.slice(0, 4).map((category) => (
                  <div key={category.id} className={styles.budgetCategoryRow}>
                    <div>
                      <span>{category.name}</span>
                      <small>{formatCurrency(category.remaining)} left</small>
                    </div>
                    <strong>{formatCurrency(category.spent)}</strong>
                  </div>
                ))}
              </div>
              <p className={styles.cardFootnote}>
                {budgetHealth.healthyCount} healthy, {budgetHealth.warningCount} nearing limit, {budgetHealth.dangerCount} over budget.
              </p>
            </article>
          </section>
        </main>

        <aside className={styles.recentRail}>
          <div className={styles.railHeader}>
            <h2>Recent Transactions</h2>
            <Link to="/transactions">See All</Link>
          </div>
          <div className={styles.filterRow}>
            <span>All transactions</span>
            <span>All accounts</span>
          </div>
          <div className={styles.recentList}>
            {recentTransactions.map((txn) => {
              const isInflow = (txn.inflow ?? 0) > 0;
              const amount = isInflow ? txn.inflow : txn.outflow;

              return (
                <article key={txn.id} className={styles.transactionCard}>
                  <div className={styles.transactionTopLine}>
                    <span>{txn.payeeName || 'Unknown Payee'}</span>
                    <time>{formatShortDate(txn.date)}</time>
                  </div>
                  <div className={styles.transactionBody}>
                    <strong>{isInflow ? '+' : '-'}{formatCurrency(amount ?? 0)}</strong>
                    <span className={styles.categoryBadge}>
                      {isInflow ? <ArrowDownLeft size={14} /> : <ArrowUpRight size={14} />}
                      <span>{txn.categoryName || 'Uncategorized'}</span>
                    </span>
                  </div>
                </article>
              );
            })}
          </div>
        </aside>
      </div>
    </div>
  );
}
