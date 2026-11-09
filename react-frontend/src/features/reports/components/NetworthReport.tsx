import { useState } from 'react';
import { Area, CartesianGrid, ComposedChart, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { TrendUp } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { DashboardCard, Segmented } from '@/features/dashboard/components/DashboardCard';
import { formatCurrency } from '@/features/dashboard/utils';
import { monthLabel } from '../cashFlow';
import { selectNetWorthLoading, selectNetWorthReport } from '../store/reportSlice';
import { AXIS_TICK, COLORS, GRID_COLOR, SERIES, axisCurrency, formatMonthTick } from '../chartTheme';
import { EmptyState, MonthTable, Stat, StatRow, TooltipCard } from './ReportParts';
import styles from './NetworthReport.module.css';

const NET_WORTH_COLOR = SERIES[0];
const CASH_COLOR = SERIES[2];

const VIEW_OPTIONS = [
  { value: 'chart', label: 'Chart' },
  { value: 'table', label: 'Table' },
] as const;

const SERIES_DEFS = [
  { key: 'netWorth', name: 'Net worth', color: NET_WORTH_COLOR },
  { key: 'cash', name: 'Cash', color: CASH_COLOR },
  { key: 'investments', name: 'Investments', color: COLORS.invested },
  { key: 'debt', name: 'Debt', color: COLORS.debt },
] as const;

interface Datum {
  month: string;
  netWorth: number;
  cash: number;
  investments: number;
  debt: number;
  loans: number;
}

function signed(value: number) {
  const rounded = Math.round(value);
  return `${rounded < 0 ? '−' : '+'}${formatCurrency(rounded)}`;
}

export default function NetworthReport() {
  const report = useAppSelector(selectNetWorthReport);
  const loading = useAppSelector(selectNetWorthLoading);
  const [view, setView] = useState<'chart' | 'table'>('chart');

  if (loading === LoadingState.PENDING || loading === LoadingState.IDLE) {
    return <EmptyState>Loading net worth…</EmptyState>;
  }
  if (!report || report.months.length === 0) {
    return <EmptyState>No data in the selected period</EmptyState>;
  }

  // drop leading months before any account existed ("All" starts in 2020)
  const firstActive = report.months.findIndex((p) => p.assets !== 0 || p.liabilities !== 0);
  const points = firstActive === -1 ? report.months : report.months.slice(firstActive);
  const data: Datum[] = points.map((point) => ({
    month: point.month,
    netWorth: point.netWorth,
    cash: point.assets - point.investments,
    investments: point.investments,
    debt: Math.abs(point.liabilities),
    loans: Math.abs(point.loans ?? 0),
  }));
  const first = data[0];
  const latest = data[data.length - 1];
  const change = latest.netWorth - first.netWorth;
  const byKey = (key: keyof Omit<Datum, 'month'>) => Object.fromEntries(data.map((d) => [d.month, d[key]]));

  return (
    <div className={styles.stack}>
      <StatRow>
        <Stat
          label="Net worth"
          color={NET_WORTH_COLOR}
          value={latest.netWorth}
          detail={
            <>
              <strong>{signed(change)}</strong> since {monthLabel(first.month)}
            </>
          }
        />
        <Stat label="Cash" color={CASH_COLOR} value={latest.cash} detail="Checking and savings accounts" />
        <Stat
          label="Investments"
          color={COLORS.invested}
          value={latest.investments}
          detail={
            <>
              <strong>{signed(latest.investments - first.investments)}</strong> since {monthLabel(first.month)}
            </>
          }
        />
        <Stat
          label="Debt"
          color={COLORS.debt}
          value={latest.debt}
          detail={
            latest.loans > 0 ? (
              <>
                <strong>{formatCurrency(latest.loans)}</strong> in loans · {formatCurrency(latest.debt - latest.loans)}{' '}
                other
              </>
            ) : (
              'Credit cards, liability and loan accounts'
            )
          }
        />
      </StatRow>

      <DashboardCard
        title="Net worth over time"
        icon={<TrendUp size={18} />}
        actions={<Segmented label="View" options={VIEW_OPTIONS} value={view} onChange={setView} />}
      >
        {view === 'chart' ? (
          <>
            <div className={styles.legend}>
              {SERIES_DEFS.map((s) => (
                <span key={s.key} className={styles.legendItem}>
                  <span className={styles.legendSwatch} style={{ background: s.color }} />
                  {s.name}
                </span>
              ))}
            </div>
            <ResponsiveContainer width="100%" height={320} initialDimension={{ width: 600, height: 320 }}>
              <ComposedChart data={data}>
                <CartesianGrid vertical={false} stroke={GRID_COLOR} />
                <XAxis
                  dataKey="month"
                  tickFormatter={formatMonthTick}
                  axisLine={{ stroke: GRID_COLOR }}
                  tickLine={false}
                  tick={AXIS_TICK}
                  minTickGap={8}
                />
                <YAxis axisLine={false} tickLine={false} width={56} tick={AXIS_TICK} tickFormatter={axisCurrency} />
                <Tooltip
                  cursor={{ stroke: 'var(--color-border)', strokeWidth: 1 }}
                  content={({ active, payload }) => {
                    const datum = payload?.[0]?.payload as Datum | undefined;
                    if (!active || !datum) return null;
                    return (
                      <TooltipCard
                        title={monthLabel(datum.month)}
                        rows={SERIES_DEFS.map((s) => ({
                          name: s.name,
                          color: s.color,
                          value: formatCurrency(datum[s.key]),
                        }))}
                      />
                    );
                  }}
                />
                <Area
                  dataKey="netWorth"
                  name="Net worth"
                  stroke={NET_WORTH_COLOR}
                  strokeWidth={2}
                  fill={NET_WORTH_COLOR}
                  fillOpacity={0.12}
                  dot={false}
                  isAnimationActive={false}
                />
                {SERIES_DEFS.slice(1).map((s) => (
                  <Line
                    key={s.key}
                    dataKey={s.key}
                    name={s.name}
                    stroke={s.color}
                    strokeWidth={2}
                    dot={false}
                    isAnimationActive={false}
                  />
                ))}
              </ComposedChart>
            </ResponsiveContainer>
          </>
        ) : (
          <MonthTable
            months={data.map((d) => d.month)}
            rows={SERIES_DEFS.map((s) => ({
              key: s.key,
              label: s.name,
              monthly: byKey(s.key),
              total: null,
              emphasis: s.key === 'netWorth',
            }))}
          />
        )}
        {latest.loans > 0 && (
          <p className={styles.caption}>
            Loan balances are worked out from each loan's rate and the payments recorded against it, so the interest
            part of every EMI isn't counted as repaid. A home bought with a loan only adds to net worth if you track it
            as an asset account.
          </p>
        )}
      </DashboardCard>
    </div>
  );
}
