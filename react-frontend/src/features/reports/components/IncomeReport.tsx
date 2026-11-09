import { useState } from 'react';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { ChartBar, HandCoins, ProhibitInset } from '@phosphor-icons/react';
import { DashboardCard, Segmented } from '@/features/dashboard/components/DashboardCard';
import { formatCurrency } from '@/features/dashboard/utils';
import { formatPercent, monthLabel, type CashFlowSummary, type IncomeSourceSummary } from '../cashFlow';
import { useCashFlowSummary } from '../hooks/useCashFlowSummary';
import { AXIS_TICK, COLORS, CURSOR_FILL, GRID_COLOR, SERIES, axisCurrency, formatMonthTick } from '../chartTheme';
import { EmptyState, MonthTable, RowBar, TooltipCard } from './ReportParts';
import styles from './IncomeReport.module.css';

const VIEW_OPTIONS = [
  { value: 'chart', label: 'Chart' },
  { value: 'table', label: 'Table' },
] as const;

// The income chart stacks fewer series than spending: past five sources the
// bars stop being readable.
const MAX_SOURCES = 5;
const OTHER_KEY = '__other__';

export default function IncomeReport() {
  const { summary, loading } = useCashFlowSummary();

  if (!summary) {
    return <EmptyState>{loading ? 'Loading income…' : 'No data in the selected period'}</EmptyState>;
  }

  const named = summary.incomeSources.slice(
    0,
    summary.incomeSources.length > MAX_SOURCES ? MAX_SOURCES - 1 : MAX_SOURCES,
  );
  const rest = summary.incomeSources.slice(named.length);
  const colorOf = (source: IncomeSourceSummary) => {
    const i = named.indexOf(source);
    return i === -1 ? COLORS.other : SERIES[i];
  };

  return (
    <div className={styles.stack}>
      {summary.incomeSources.length > 0 ? (
        <>
          <SourcesCard summary={summary} colorOf={colorOf} />
          <TrendCard summary={summary} named={named} rest={rest} />
        </>
      ) : (
        <DashboardCard title="Income sources" icon={<HandCoins size={18} />}>
          <EmptyState>No earned income in the selected period</EmptyState>
        </DashboardCard>
      )}
      <NotIncomeCard summary={summary} />
    </div>
  );
}

function SourcesCard({
  summary,
  colorOf,
}: {
  summary: CashFlowSummary;
  colorOf: (source: IncomeSourceSummary) => string;
}) {
  const monthCount = Math.max(summary.months.length, 1);
  const max = Math.max(...summary.incomeSources.map((source) => source.total), 1);

  return (
    <DashboardCard title="Income sources" icon={<HandCoins size={18} />}>
      <ul className={styles.list}>
        <li className={styles.headRow} aria-hidden="true">
          <span>Source</span>
          <span />
          <span className={styles.num}>Total</span>
          <span className={styles.num}>Share</span>
          <span className={styles.num}>Per month</span>
          <span className={styles.num}>Months paid</span>
        </li>
        {summary.incomeSources.map((source) => (
          <li key={source.id} className={styles.row}>
            <span className={styles.name}>
              <span className={styles.dot} style={{ background: colorOf(source) }} />
              <span className={styles.nameText}>{source.name}</span>
            </span>
            <RowBar value={source.total} max={max} color={colorOf(source)} />
            <span className={`${styles.num} ${styles.strong}`}>{formatCurrency(source.total)}</span>
            <span className={styles.num}>{formatPercent((source.total / summary.income) * 100)}</span>
            <span className={styles.num}>{formatCurrency(source.total / monthCount)}</span>
            <span className={styles.num}>
              {source.activeMonths} of {summary.months.length}
            </span>
          </li>
        ))}
      </ul>
    </DashboardCard>
  );
}

function TrendCard({
  summary,
  named,
  rest,
}: {
  summary: CashFlowSummary;
  named: IncomeSourceSummary[];
  rest: IncomeSourceSummary[];
}) {
  const [view, setView] = useState<'chart' | 'table'>('chart');
  const series = [
    ...named.map((source, i) => ({ key: source.id, name: source.name, color: SERIES[i] })),
    ...(rest.length > 0 ? [{ key: OTHER_KEY, name: `Other (${rest.length})`, color: COLORS.other }] : []),
  ];
  const data = summary.months.map((month) => {
    const row: Record<string, number | string> = { month };
    for (const source of named) row[source.id] = Math.max(source.monthly[month] ?? 0, 0);
    if (rest.length > 0) {
      row[OTHER_KEY] = Math.max(
        rest.reduce((sum, source) => sum + (source.monthly[month] ?? 0), 0),
        0,
      );
    }
    return row;
  });

  return (
    <DashboardCard
      title="Income by month"
      icon={<ChartBar size={18} />}
      actions={<Segmented label="View" options={VIEW_OPTIONS} value={view} onChange={setView} />}
    >
      {view === 'chart' ? (
        <>
          <div className={styles.legend}>
            {series.map((s) => (
              <span key={s.key} className={styles.legendItem}>
                <span className={styles.dot} style={{ background: s.color }} />
                {s.name}
              </span>
            ))}
          </div>
          <ResponsiveContainer width="100%" height={280} initialDimension={{ width: 600, height: 280 }}>
            <BarChart data={data} barCategoryGap="24%">
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
                cursor={{ fill: CURSOR_FILL }}
                content={({ active, payload, label }) => {
                  if (!active || !payload?.length) return null;
                  const rows = [...payload]
                    .filter((entry) => Number(entry.value) > 0)
                    .reverse()
                    .map((entry) => ({
                      name: String(entry.name),
                      color: String(entry.color),
                      value: formatCurrency(Number(entry.value)),
                    }));
                  const total = payload.reduce((sum, entry) => sum + Number(entry.value ?? 0), 0);
                  return (
                    <TooltipCard
                      title={monthLabel(String(label))}
                      rows={rows}
                      footer={`Total ${formatCurrency(total)}`}
                    />
                  );
                }}
              />
              {series.map((s, i) => (
                <Bar
                  key={s.key}
                  dataKey={s.key}
                  name={s.name}
                  stackId="income"
                  fill={s.color}
                  stroke="var(--dash-surface)"
                  strokeWidth={1}
                  radius={i === series.length - 1 ? [4, 4, 0, 0] : 0}
                  maxBarSize={28}
                  isAnimationActive={false}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        </>
      ) : (
        <MonthTable
          months={summary.months}
          rows={[
            ...summary.incomeSources.map((source) => ({ key: source.id, label: source.name, monthly: source.monthly })),
            { key: 'total', label: 'Total', monthly: summary.incomeMonthly, emphasis: true },
          ]}
        />
      )}
    </DashboardCard>
  );
}

/** Money that reached the budget without being earned. */
function NotIncomeCard({ summary }: { summary: CashFlowSummary }) {
  const withdrawals = summary.assetAccounts.filter((account) => account.in > 0.5);
  const borrowing = summary.debtAccounts.filter((account) => account.in > 0.5);
  if (withdrawals.length === 0 && borrowing.length === 0 && summary.opening <= 0) return null;

  return (
    <DashboardCard title="Came in, but not counted as income" icon={<ProhibitInset size={18} />}>
      <p className={styles.help}>
        This money landed in your budget accounts, but it wasn't earned, so it's left out of income and the spend rate.
      </p>
      <ul className={styles.plainList}>
        {withdrawals.map((account) => (
          <li key={account.id} className={styles.plainRow}>
            <span className={styles.name}>
              <span className={styles.dot} style={{ background: COLORS.invested }} />
              <span className={styles.nameText}>
                Taken out of {account.name}
                {account.recategorizedCount > 0 && (
                  <span className={styles.subtle}> · {account.recategorizedCount} filed under a category</span>
                )}
              </span>
            </span>
            <span className={`${styles.num} ${styles.strong}`}>{formatCurrency(account.in)}</span>
          </li>
        ))}
        {borrowing.map((account) => (
          <li key={account.id} className={styles.plainRow}>
            <span className={styles.name}>
              <span className={styles.dot} style={{ background: COLORS.debt }} />
              <span className={styles.nameText}>Borrowed from {account.name}</span>
            </span>
            <span className={`${styles.num} ${styles.strong}`}>{formatCurrency(account.in)}</span>
          </li>
        ))}
        {summary.opening > 0 && (
          <li className={styles.plainRow}>
            <span className={styles.name}>
              <span className={styles.dot} style={{ background: COLORS.cash }} />
              <span className={styles.nameText}>Opening balances</span>
            </span>
            <span className={`${styles.num} ${styles.strong}`}>{formatCurrency(summary.opening)}</span>
          </li>
        )}
      </ul>
    </DashboardCard>
  );
}
