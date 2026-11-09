import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowDownLeft, ArrowRight, ArrowsLeftRight, Receipt, Sparkle } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { TransactionStatus, type Transaction } from '@/features/transactions/types/transaction.types';
import { getTodaysDate } from '@/utils/date.utils';
import { selectDashboardActivity, selectNeedsReviewCount } from '../../store/dashboardSlice';
import { formatRelativeDay, formatSignedCurrency, transactionNet } from '../../utils';
import { DashboardCard, Segmented } from '../DashboardCard';
import styles from './RecentActivity.module.css';

const RECENT_LIMIT = 40;

const FILTERS = [
  { value: 'all', label: 'All' },
  { value: 'out', label: 'Spending' },
  { value: 'in', label: 'Income' },
] as const;

type Filter = (typeof FILTERS)[number]['value'];

const matchesFilter = (txn: Transaction, filter: Filter) => {
  if (filter === 'all') return true;
  if (txn.transferAccountId) return false;
  return filter === 'in' ? (txn.inflow ?? 0) > 0 : (txn.outflow ?? 0) > 0;
};

/** What the row says and shows, by kind of transaction. */
function describe(txn: Transaction, inflowCategoryId: string | undefined, accountNames: Map<string, string>) {
  const net = transactionNet(txn);
  if (txn.transferAccountId) {
    const other = accountNames.get(txn.transferAccountId) ?? 'another account';
    return {
      kind: 'transfer',
      Icon: ArrowsLeftRight,
      title: `${net < 0 ? 'To' : 'From'} ${other}`,
      detail: 'Transfer',
    } as const;
  }
  const title = txn.payeeName || 'Unknown payee';
  if (inflowCategoryId && txn.categoryId === inflowCategoryId) {
    return { kind: 'income', Icon: ArrowDownLeft, title, detail: 'Ready to assign' } as const;
  }
  return {
    kind: txn.categoryId ? 'spend' : 'uncategorized',
    Icon: null,
    title,
    detail: txn.categoryName || 'Uncategorized',
  } as const;
}

const initial = (name: string) => Array.from(name.trim())[0]?.toUpperCase() ?? '?';

export default function RecentActivity({ className = '' }: { className?: string }) {
  const activity = useAppSelector(selectDashboardActivity);
  const needsReview = useAppSelector(selectNeedsReviewCount);
  const inflowCategoryId = useAppSelector((state) => state.budgets.selectedBudget?.metadata?.inflowCategoryId);
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const accountNames = useMemo(
    () => new Map(accounts.flatMap((account) => (account.id ? [[account.id, account.name] as const] : []))),
    [accounts],
  );
  const [filter, setFilter] = useState<Filter>('all');
  const today = getTodaysDate();

  const days = useMemo(() => {
    const grouped: Array<{ date: string; transactions: Transaction[] }> = [];
    const recent = activity.filter((txn) => txn.date <= today && matchesFilter(txn, filter)).slice(0, RECENT_LIMIT);
    for (const txn of recent) {
      const last = grouped[grouped.length - 1];
      if (last?.date === txn.date) last.transactions.push(txn);
      else grouped.push({ date: txn.date, transactions: [txn] });
    }
    return grouped;
  }, [activity, filter, today]);

  return (
    <DashboardCard
      title="Recent activity"
      icon={<Receipt size={18} />}
      className={`${styles.card} ${className}`}
      actions={
        <Link to="/transactions" className={styles.seeAll}>
          See all <ArrowRight size={13} />
        </Link>
      }
    >
      <div className={styles.toolbar}>
        <Segmented label="Show" options={FILTERS} value={filter} onChange={setFilter} />
        {needsReview > 0 && (
          <Link to="/settings?section=review" className={styles.reviewLink}>
            <Sparkle size={13} weight="fill" aria-hidden="true" />
            {needsReview} to review
          </Link>
        )}
      </div>

      {days.length === 0 ? (
        <p className={styles.empty}>
          {filter === 'all' ? 'No transactions yet.' : `No ${filter === 'in' ? 'income' : 'spending'} lately.`}
        </p>
      ) : (
        <div className={styles.scroller}>
          {days.map((day) => (
            <section key={day.date} className={styles.day} aria-label={formatRelativeDay(day.date, today)}>
              <h3 className={styles.dayLabel}>{formatRelativeDay(day.date, today)}</h3>
              <ul className={styles.list}>
                {day.transactions.map((txn) => {
                  const { kind, Icon, title, detail } = describe(txn, inflowCategoryId, accountNames);
                  const net = transactionNet(txn);
                  return (
                    <li key={txn.id}>
                      <Link to={`/transactions/${txn.accountId}`} className={styles.row}>
                        <span className={styles.rowIcon} data-kind={kind}>
                          {Icon ? (
                            <Icon size={17} aria-hidden="true" />
                          ) : (
                            <span aria-hidden="true">{initial(title)}</span>
                          )}
                        </span>
                        <span className={styles.rowBody}>
                          <span className={styles.payee}>{title}</span>
                          <span className={styles.meta}>
                            <span className={kind === 'uncategorized' ? styles.metaWarn : undefined}>{detail}</span>
                            {' · '}
                            {txn.accountName}
                          </span>
                        </span>
                        <span className={styles.rowEnd}>
                          <span className={styles.amount} data-kind={kind}>
                            {formatSignedCurrency(net, kind !== 'transfer')}
                          </span>
                          {txn.status === TransactionStatus.UNAPPROVED && (
                            <span className={styles.reviewBadge}>
                              <Sparkle size={10} weight="fill" aria-hidden="true" />
                              Review
                            </span>
                          )}
                        </span>
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </section>
          ))}
        </div>
      )}
    </DashboardCard>
  );
}
