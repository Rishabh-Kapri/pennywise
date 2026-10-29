import { useState } from 'react';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { ChartBar, Info, Scales } from '@phosphor-icons/react';
import { DashboardCard, Segmented } from '@/features/dashboard/components/DashboardCard';
import { formatCurrency } from '@/features/dashboard/utils';
import { formatPercent, monthLabel, shareOfIncome, type CashFlowSummary } from '../cashFlow';
import { useCashFlowSummary } from '../hooks/useCashFlowSummary';
import { AXIS_TICK, COLORS, CURSOR_FILL, GRID_COLOR, axisCurrency, formatMonthTick } from '../chartTheme';
import { EmptyState, FlowBar, MonthTable, Note, Stat, StatRow, TooltipCard, type FlowSegment } from './ReportParts';
import styles from './OverviewReport.module.css';

type View = 'chart' | 'table';
const VIEW_OPTIONS = [
  { value: 'chart', label: 'Chart' },
  { value: 'table', label: 'Table' },
] as const;

export default function OverviewReport() {
  const { summary, loading } = useCashFlowSummary();

  if (!summary) {
    return <EmptyState>{loading ? 'Crunching the numbers…' : 'No data in the selected period'}</EmptyState>;
  }

  const totalSpent = summary.spending + summary.oneOff;
  const monthCount = Math.max(summary.months.length, 1);
  const spentShare = shareOfIncome(totalSpent, summary.income);
  const coreShare = shareOfIncome(summary.spending, summary.income);
  const leftOver = summary.income - totalSpent;
  const netInvested = summary.invested - summary.withdrawn;

  return (
    <div className={styles.stack}>
      <StatRow>
        <Stat
          label="Earned income"
          color={COLORS.income}
          value={summary.income}
          detail={
            <>
              <strong>{formatCurrency(summary.income / monthCount)}</strong> a month on average
            </>
          }
        />
        <Stat
          label="Spent"
          color={COLORS.spending}
          value={totalSpent}
          detail={
            <>
              <strong>{formatPercent(spentShare)}</strong> of income
              {summary.oneOff > 0 && (
                <>
                  {' · '}
                  <strong>{formatPercent(coreShare)}</strong> without one-offs
                </>
              )}
            </>
          }
        />
        <Stat
          label={leftOver >= 0 ? 'Left over' : 'Spent beyond income'}
          value={Math.abs(leftOver)}
          detail={
            summary.income > 0 ? (
              <>
                Savings rate <strong>{formatPercent(shareOfIncome(leftOver, summary.income))}</strong>
              </>
            ) : (
              'No earned income in this period'
            )
          }
        />
        <Stat
          label={netInvested >= 0 ? 'Net invested' : 'Net withdrawn'}
          color={COLORS.invested}
          value={Math.abs(netInvested)}
          detail={
            <>
              <strong>{formatCurrency(summary.invested)}</strong> in ·{' '}
              <strong>{formatCurrency(summary.withdrawn)}</strong> out
            </>
          }
        />
      </StatRow>

      <DashboardCard title="Where the money came from and went" icon={<Scales size={18} />}>
        <MoneyFlow summary={summary} />
      </DashboardCard>

      <MonthlyCard summary={summary} />

      <TransferNote summary={summary} />
    </div>
  );
}

function MoneyFlow({ summary }: { summary: CashFlowSummary }) {
  const sources: FlowSegment[] = [
    { key: 'income', label: 'Earned income', value: summary.income, color: COLORS.income },
    {
      key: 'withdrawn',
      label: 'Taken out of investments',
      hint: 'Sold funds, stocks, FDs and other asset accounts',
      value: summary.withdrawn,
      color: COLORS.invested,
    },
    { key: 'borrowed', label: 'Borrowed', value: summary.borrowed, color: COLORS.debt },
    {
      key: 'cash',
      label: 'From account balances',
      hint: 'Spent out of money you already had',
      value: Math.max(-summary.cashChange, 0),
      color: COLORS.cash,
    },
  ];
  const uses: FlowSegment[] = [
    { key: 'spending', label: 'Spending', value: summary.spending, color: COLORS.spending },
    { key: 'oneOff', label: 'One-off spending', value: summary.oneOff, color: COLORS.oneOff },
    {
      key: 'invested',
      label: 'Saved & invested',
      hint:
        summary.savedInGroups > 0 ? `Includes ${formatCurrency(summary.savedInGroups)} from saving groups` : undefined,
      value: summary.invested,
      color: COLORS.invested,
    },
    { key: 'debt', label: 'Debt payments', value: summary.debtPaid, color: COLORS.debt },
    {
      key: 'cash',
      label: 'Kept in accounts',
      value: Math.max(summary.cashChange, 0),
      color: COLORS.cash,
    },
  ];
  const scale = Math.max(
    sources.reduce((sum, s) => sum + Math.max(s.value, 0), 0),
    uses.reduce((sum, s) => sum + Math.max(s.value, 0), 0),
    1,
  );

  return (
    <div className={styles.flowLayout}>
      <p className={styles.insight}>{insight(summary)}</p>
      <div className={styles.flowColumns}>
        <FlowBar title="Money in" segments={sources} scale={scale} income={summary.income} />
        <FlowBar title="Money out" segments={uses} scale={scale} income={summary.income} />
      </div>
      <p className={styles.caption}>Percentages are of earned income.</p>
    </div>
  );
}

/** One or two plain sentences that say what the bars show. */
function insight(summary: CashFlowSummary): string {
  const totalSpent = summary.spending + summary.oneOff;
  if (summary.income <= 0) {
    return totalSpent > 0
      ? `No earned income was recorded, and ${formatCurrency(totalSpent)} was spent.`
      : 'Nothing was earned or spent in this period.';
  }
  const parts: string[] = [];
  const share = formatPercent(shareOfIncome(totalSpent, summary.income));
  if (totalSpent <= summary.income) {
    parts.push(
      `You spent ${share} of what you earned and kept ${formatPercent(shareOfIncome(summary.income - totalSpent, summary.income))}.`,
    );
  } else {
    const funders = [
      summary.withdrawn > 0 && `${formatCurrency(summary.withdrawn)} taken out of investments`,
      summary.borrowed > 0 && `${formatCurrency(summary.borrowed)} borrowed`,
    ].filter(Boolean);
    parts.push(
      `You spent ${share} of what you earned — ${formatCurrency(totalSpent - summary.income)} more than came in` +
        (funders.length > 0 ? `, covered by ${funders.join(' and ')}.` : '.'),
    );
  }
  if (summary.oneOff > 0) {
    parts.push(
      `Leaving out ${formatCurrency(summary.oneOff)} of one-off spending, everyday spending was ${formatPercent(shareOfIncome(summary.spending, summary.income))} of income.`,
    );
  }
  return parts.join(' ');
}

interface MonthDatum {
  month: string;
  income: number;
  spending: number;
  oneOff: number;
}

function MonthlyCard({ summary }: { summary: CashFlowSummary }) {
  const [view, setView] = useState<View>('chart');
  // one big purchase (a house) flattens every other month; let it be hidden
  const [everydayOnly, setEverydayOnly] = useState(false);
  const data: MonthDatum[] = summary.months.map((month) => ({
    month,
    income: summary.incomeMonthly[month] ?? 0,
    // a net-refund month shows as zero spending rather than a negative bar
    spending: Math.max(summary.spendingMonthly[month] ?? 0, 0),
    oneOff: everydayOnly ? 0 : Math.max(summary.oneOffMonthly[month] ?? 0, 0),
  }));
  const hasOneOff = summary.oneOff > 0;
  const showOneOff = hasOneOff && !everydayOnly;

  const rate = (month: string | null) => {
    const income = month ? (summary.incomeMonthly[month] ?? 0) : summary.income;
    const spent = month
      ? (summary.spendingMonthly[month] ?? 0) + (summary.oneOffMonthly[month] ?? 0)
      : summary.spending + summary.oneOff;
    return formatPercent(shareOfIncome(spent, income));
  };

  return (
    <DashboardCard
      title="Month by month"
      icon={<ChartBar size={18} />}
      actions={<Segmented label="View" options={VIEW_OPTIONS} value={view} onChange={setView} />}
    >
      {view === 'chart' ? (
        <>
          <div className={styles.chartBar}>
            <div className={styles.legend}>
              <LegendItem color={COLORS.income} label="Earned income" />
              <LegendItem color={COLORS.spending} label="Spending" />
              {showOneOff && <LegendItem color={COLORS.oneOff} label="One-off" />}
            </div>
            {hasOneOff && (
              <label className={styles.toggle}>
                <input type="checkbox" checked={everydayOnly} onChange={(e) => setEverydayOnly(e.target.checked)} />
                Hide one-offs
              </label>
            )}
          </div>
          <ResponsiveContainer width="100%" height={280} initialDimension={{ width: 600, height: 280 }}>
            <BarChart data={data} barGap={2} barCategoryGap="24%">
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
                content={({ active, payload }) => {
                  const datum = payload?.[0]?.payload as MonthDatum | undefined;
                  if (!active || !datum) return null;
                  return (
                    <TooltipCard
                      title={monthLabel(datum.month)}
                      rows={[
                        { name: 'Earned income', color: COLORS.income, value: formatCurrency(datum.income) },
                        { name: 'Spending', color: COLORS.spending, value: formatCurrency(datum.spending) },
                        ...(hasOneOff
                          ? [{ name: 'One-off', color: COLORS.oneOff, value: formatCurrency(datum.oneOff) }]
                          : []),
                      ]}
                      footer={`Spent ${rate(datum.month)} of income`}
                    />
                  );
                }}
              />
              <Bar
                dataKey="income"
                fill={COLORS.income}
                radius={[4, 4, 0, 0]}
                maxBarSize={18}
                isAnimationActive={false}
              />
              <Bar
                dataKey="spending"
                stackId="out"
                fill={COLORS.spending}
                radius={showOneOff ? 0 : [4, 4, 0, 0]}
                maxBarSize={18}
                isAnimationActive={false}
              />
              <Bar
                dataKey="oneOff"
                stackId="out"
                fill={COLORS.oneOff}
                radius={[4, 4, 0, 0]}
                maxBarSize={18}
                isAnimationActive={false}
              />
            </BarChart>
          </ResponsiveContainer>
        </>
      ) : (
        <MonthTable
          months={summary.months}
          rows={[
            { key: 'income', label: 'Earned income', monthly: summary.incomeMonthly },
            { key: 'spending', label: 'Spending', monthly: summary.spendingMonthly },
            ...(hasOneOff ? [{ key: 'oneOff', label: 'One-off', monthly: summary.oneOffMonthly }] : []),
            {
              key: 'rate',
              label: 'Spent of income',
              monthly: {},
              format: (_value: number, month: string | null) => rate(month),
              total: 0,
              emphasis: true,
            },
            {
              key: 'invested',
              label: 'Net invested',
              monthly: Object.fromEntries(
                summary.months.map((m) => [m, (summary.investedMonthly[m] ?? 0) - (summary.withdrawnMonthly[m] ?? 0)]),
              ),
            },
          ]}
        />
      )}
    </DashboardCard>
  );
}

function LegendItem({ color, label }: { color: string; label: string }) {
  return (
    <span className={styles.legendItem}>
      <span className={styles.legendSwatch} style={{ background: color }} />
      {label}
    </span>
  );
}

/** Explains why the totals differ from what the categories alone would say. */
function TransferNote({ summary }: { summary: CashFlowSummary }) {
  const hasTransfers = summary.assetAccounts.length > 0 || summary.debtAccounts.length > 0;
  if (!hasTransfers && summary.opening <= 0) return null;

  return (
    <Note icon={<Info size={18} />}>
      {hasTransfers && (
        <p>
          Money moved to or from asset, loan and liability accounts is never counted as income or spending, whichever
          category it was filed under. Selling an investment shows as <strong>taken out of investments</strong>; buying
          one shows as <strong>saved &amp; invested</strong>.
        </p>
      )}
      {summary.recategorizedCount > 0 && (
        <p>
          {summary.recategorizedCount === 1 ? 'One transfer' : `${summary.recategorizedCount} transfers`} out of asset
          accounts ({formatCurrency(summary.recategorizedAmount)}) went straight to a category instead of Ready to
          Assign. {summary.recategorizedCount === 1 ? "It's" : "They're"} counted as withdrawals here too, so they don't
          hide spending in those categories. Nothing to clean up.
        </p>
      )}
      {summary.opening > 0 && <p>Opening balances ({formatCurrency(summary.opening)}) aren't counted as income.</p>}
    </Note>
  );
}
