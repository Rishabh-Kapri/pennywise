import { ArrowDown, ArrowDownLeft, ArrowUp, ArrowUpRight, ArrowsDownUp } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { addMonths } from '@/utils/date.utils';
import { selectCashFlowRefreshing, selectMonthCashFlow } from '../../store/dashboardSlice';
import { formatCompactCurrency, formatCurrency, formatMonthName } from '../../utils';
import { Amount } from '../Amount';
import { DashboardCard } from '../DashboardCard';
import styles from './CashFlowCard.module.css';

interface ChangeProps {
  current: number;
  previous: number | null;
  upIsGood: boolean;
  against: string;
}

/** Month-on-month change, worded so it never relies on color alone. */
const Change = ({ current, previous, upIsGood, against }: ChangeProps) => {
  if (previous === null) return null;
  const diff = current - previous;
  if (Math.round(diff) === 0) {
    return <span className={`${styles.change} ${styles.changeFlat}`}>Same as {against}</span>;
  }
  const up = diff > 0;

  return (
    <span className={`${styles.change} ${up === upIsGood ? styles.changeGood : styles.changeBad}`}>
      {up ? <ArrowUp size={12} weight="bold" /> : <ArrowDown size={12} weight="bold" />}
      {formatCompactCurrency(Math.abs(diff))}
      <span className={styles.changeContext}>vs {against}</span>
    </span>
  );
};

export default function CashFlowCard({ className = '' }: { className?: string }) {
  const flow = useAppSelector(selectMonthCashFlow);
  const refreshing = useAppSelector(selectCashFlowRefreshing);

  if (!flow) {
    return (
      <DashboardCard title="Cash flow" icon={<ArrowsDownUp size={18} />} className={className}>
        <p className={styles.empty}>Cash flow will show up once there are transactions.</p>
      </DashboardCard>
    );
  }

  const prevName = formatMonthName(addMonths(flow.month, -1), 'short');
  const spentShare = flow.income > 0 ? flow.spending / flow.income : null;
  const overspent = flow.net < 0;

  return (
    <DashboardCard
      title="Cash flow"
      icon={<ArrowsDownUp size={18} />}
      className={className}
      refreshing={refreshing}
      actions={<span className={styles.month}>{formatMonthName(flow.month)}</span>}
    >
      <dl className={styles.flows}>
        <div className={styles.flow}>
          <dt className={styles.flowLabel}>
            <span className={`${styles.flowIcon} ${styles.flowIconIn}`}>
              <ArrowDownLeft size={15} weight="bold" />
            </span>
            Income
          </dt>
          <dd className={styles.flowValue}>
            <Amount value={flow.income} signed className={styles.flowAmount} />
            <Change current={flow.income} previous={flow.prevIncome} upIsGood against={prevName} />
          </dd>
        </div>
        <div className={styles.flow}>
          <dt className={styles.flowLabel}>
            <span className={styles.flowIcon}>
              <ArrowUpRight size={15} weight="bold" />
            </span>
            Spending
          </dt>
          <dd className={styles.flowValue}>
            <Amount value={-flow.spending} className={styles.flowAmount} />
            <Change current={flow.spending} previous={flow.prevSpending} upIsGood={false} against={prevName} />
          </dd>
        </div>
      </dl>

      <div className={styles.net}>
        <div className={styles.netTop}>
          <span className={styles.netLabel}>{overspent ? 'Overspent' : 'Left over'}</span>
          <Amount
            value={Math.abs(flow.net)}
            className={`${styles.netAmount} ${overspent ? styles.netNegative : ''}`}
          />
        </div>
        <div
          className={styles.meter}
          role="meter"
          aria-label="Share of income spent"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={spentShare === null ? 0 : Math.round(Math.min(spentShare, 1) * 100)}
        >
          <div
            className={`${styles.meterFill} ${spentShare !== null && spentShare > 1 ? styles.meterOver : ''}`}
            style={{ width: `${spentShare === null ? 0 : Math.min(spentShare, 1) * 100}%` }}
          />
        </div>
        <p className={styles.meterText}>
          {spentShare === null
            ? flow.spending > 0
              ? 'No income recorded this month'
              : 'Nothing in or out yet'
            : spentShare <= 1
              ? `${Math.round(spentShare * 100)}% of income spent`
              : `Spent ${formatCurrency(flow.spending - flow.income)} more than came in`}
        </p>
      </div>

      <footer className={styles.average}>
        {flow.avgMonths > 0 ? (
          <>
            <span className={styles.averageLabel}>
              Monthly average, last {flow.avgMonths === 1 ? 'month' : `${flow.avgMonths} months`}
            </span>
            <span className={styles.averageValues}>
              <span>
                <strong>+{formatCurrency(flow.avgIncome)}</strong> in
              </span>
              <span>
                <strong>−{formatCurrency(flow.avgSpending)}</strong> out
              </span>
            </span>
          </>
        ) : (
          <span className={styles.averageLabel}>Averages appear after your first full month.</span>
        )}
      </footer>
    </DashboardCard>
  );
}
