import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, Check, TrendDown, TrendUp, Wallet } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { BankLogo } from '@/features/accounts/components/BankLogo/BankLogo';
import { BudgetAccountNames, TrackingAccountNames } from '@/features/accounts/types/account.types';
import { getTodaysDate } from '@/utils/date.utils';
import {
  ACTIVITY_DAYS,
  selectActivityWindow,
  selectDashboardAccounts,
  selectDashboardActivity,
} from '../../store/dashboardSlice';
import { useExcludedAccounts } from '../../hooks/useExcludedAccounts';
import { addDays, buildBalanceHistory, formatCurrency, formatDayMonth, formatSignedCurrency } from '../../utils';
import { Amount } from '../Amount';
import { DashboardCard, Segmented } from '../DashboardCard';
import BalanceChart from './BalanceChart';
import styles from './BalanceCard.module.css';

const RANGES = [
  { value: '1m', label: '1M', days: 30 },
  { value: '3m', label: '3M', days: 91 },
  { value: '6m', label: '6M', days: ACTIVITY_DAYS },
] as const;

type Range = (typeof RANGES)[number]['value'];

const TYPE_LABELS: Record<string, string> = Object.fromEntries(
  [...BudgetAccountNames, ...TrackingAccountNames].map(({ value, name }) => [value, name]),
);

export default function BalanceCard({ className = '' }: { className?: string }) {
  const accounts = useAppSelector(selectDashboardAccounts);
  const activity = useAppSelector(selectDashboardActivity);
  const activityWindow = useAppSelector(selectActivityWindow);
  const budgetId = useAppSelector((state) => state.budgets.selectedBudget?.id);
  const [range, setRange] = useState<Range>('3m');
  const [excluded, toggleAccount] = useExcludedAccounts(budgetId);
  const today = getTodaysDate();

  const { includedIds, total } = useMemo(() => {
    const ids = new Set<string>();
    let sum = 0;
    for (const account of accounts) {
      if (!account.id || excluded.has(account.id)) continue;
      ids.add(account.id);
      sum += account.balance ?? 0;
    }
    return { includedIds: ids, total: sum };
  }, [accounts, excluded]);

  const history = useMemo(() => {
    if (!activityWindow.start || includedIds.size === 0) return [];
    const days = RANGES.find((option) => option.value === range)?.days ?? ACTIVITY_DAYS;
    const rangeStart = addDays(today, -days);
    const start = rangeStart > activityWindow.reliableStart ? rangeStart : activityWindow.reliableStart;
    return buildBalanceHistory(total, activity, includedIds, start, today);
  }, [activity, activityWindow, includedIds, range, today, total]);

  const first = history[0];
  const change = first ? total - first.balance : 0;
  const percent = first && first.balance !== 0 ? (change / Math.abs(first.balance)) * 100 : null;
  const includedCount = includedIds.size;

  return (
    <DashboardCard
      title="Balance"
      icon={<Wallet size={18} />}
      className={className}
      actions={
        accounts.length > 0 && (
          <Segmented label="Chart range" options={RANGES} value={range} onChange={setRange} />
        )
      }
    >
      {accounts.length === 0 ? (
        <div className={styles.empty}>
          <p>Add an account to see your balance here.</p>
          <Link to="/accounts" className={styles.manageLink}>
            Go to accounts <ArrowRight size={14} />
          </Link>
        </div>
      ) : (
        <div className={styles.body}>
          <div className={styles.accounts}>
            <ul className={styles.accountList} aria-label="Accounts in the balance">
              {accounts.map((account) => {
                const id = account.id ?? '';
                const included = includedIds.has(id);
                const balance = account.balance ?? 0;
                return (
                  <li key={id}>
                    <button
                      type="button"
                      className={styles.accountRow}
                      aria-pressed={included}
                      title={included ? 'Leave out of the balance' : 'Count in the balance'}
                      onClick={() => toggleAccount(id)}
                    >
                      <span className={styles.check} aria-hidden="true">
                        {included && <Check size={10} weight="bold" />}
                      </span>
                      <BankLogo name={account.name} bank={account.bank} type={account.type} size="sm" />
                      <span className={styles.accountText}>
                        <span className={styles.accountName}>{account.name}</span>
                        <span className={styles.accountType}>{TYPE_LABELS[account.type] ?? 'Account'}</span>
                      </span>
                      <span className={`${styles.accountBalance} ${balance < 0 ? styles.negative : ''}`}>
                        {formatSignedCurrency(balance)}
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
            <Link to="/accounts" className={styles.manageLink}>
              Manage accounts <ArrowRight size={14} />
            </Link>
          </div>

          <div className={styles.overview}>
            <div className={styles.hero}>
              <span className={styles.heroLabel}>
                {includedCount === accounts.length
                  ? `Across ${accounts.length === 1 ? 'your account' : `all ${accounts.length} accounts`}`
                  : `Across ${includedCount} of ${accounts.length} accounts`}
              </span>
              <Amount value={total} className={styles.heroAmount} />
              {first && history.length > 1 && (
                <span className={styles.change}>
                  {Math.round(change) === 0 ? (
                    <span className={styles.changeFlat}>No change</span>
                  ) : (
                    <span className={change > 0 ? styles.changeUp : styles.changeDown}>
                      {change > 0 ? <TrendUp size={15} weight="bold" /> : <TrendDown size={15} weight="bold" />}
                      {change > 0 ? '+' : '−'}
                      {formatCurrency(change)}
                      {percent !== null && Math.abs(percent) >= 0.1 && ` (${Math.abs(percent).toFixed(1)}%)`}
                    </span>
                  )}
                  <span className={styles.changeContext}>since {formatDayMonth(first.date)}</span>
                </span>
              )}
            </div>

            {includedCount === 0 ? (
              <div className={styles.chartEmpty}>Tick an account to chart its balance</div>
            ) : (
              <BalanceChart data={history} />
            )}
          </div>
        </div>
      )}
    </DashboardCard>
  );
}
