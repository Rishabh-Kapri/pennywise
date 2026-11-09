import { useMemo, useState, type ReactNode } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';
import { ChevronDown, ChevronRight, Info } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { BarChart, Legend, LineChart, RowBar, Segmented, StackBar, Stat, type ChartSeries } from '../../../components/charts';
import { LoadingStateView } from '../../../components/LoadingStateView';
import { chartColors, chartSeries, colors, radii, spacing } from '../../../theme';
import { LoadingState } from '../../../utils/constants';
import { formatCurrency, formatSignedCurrency } from '../../../utils/date';
import {
  defaultTreatment,
  formatPercent,
  monthLabel,
  shareOfIncome,
  summarizeCashFlow,
  type CashFlowSummary,
  type GroupSummary,
  type Monthly
} from '../cashFlow';
import { setGroupTreatment } from '../store/reportSlice';
import type { GroupTreatment } from '../types';

// Mobile take on react-frontend's Reports tabs: same numbers from the same
// `reports/cashflow` classification, with tap-to-inspect charts and lists in
// place of the wide month tables.

function useCashFlowSummary() {
  const report = useAppSelector((state) => state.reports.cashFlow);
  const loading = useAppSelector((state) => state.reports.cashFlowLoading);
  const treatments = useAppSelector((state) => state.reports.groupTreatments);
  const summary = useMemo(() => (report ? summarizeCashFlow(report, treatments) : null), [report, treatments]);
  return { summary, loading: loading === LoadingState.PENDING || loading === LoadingState.IDLE };
}

function Empty({ children }: { children: ReactNode }) {
  return (
    <Card style={styles.empty}>
      <AppText muted style={styles.center}>
        {children}
      </AppText>
    </Card>
  );
}

function Section({ title, right, children }: { title: string; right?: ReactNode; children: ReactNode }) {
  return (
    <Card style={styles.section}>
      <View style={styles.sectionHead}>
        <AppText variant="heading" style={styles.grow}>
          {title}
        </AppText>
        {right}
      </View>
      {children}
    </Card>
  );
}

function Note({ children }: { children: ReactNode }) {
  return (
    <View style={styles.note}>
      <Info size={16} color={colors.muted} />
      <View style={styles.grow}>{children}</View>
    </View>
  );
}

/** Line items for one selected month under a chart. */
function MonthDetail({ title, rows, footer }: { title: string; rows: { label: string; color?: string; value: string }[]; footer?: string }) {
  return (
    <View style={styles.detail}>
      <AppText variant="caption" weight="semibold">
        {title}
      </AppText>
      {rows.map((row) => (
        <View key={row.label} style={styles.detailRow}>
          {row.color ? <View style={[styles.dot, { backgroundColor: row.color }]} /> : null}
          <AppText variant="caption" muted style={styles.grow} numberOfLines={1}>
            {row.label}
          </AppText>
          <AppText variant="caption" weight="semibold" tabular>
            {row.value}
          </AppText>
        </View>
      ))}
      {footer ? (
        <AppText variant="caption" tone="faint">
          {footer}
        </AppText>
      ) : null}
    </View>
  );
}

const toggle = (setter: (fn: (current: number | null) => number | null) => void) => (index: number) =>
  setter((current) => (current === index ? null : index));

// ── Overview ──

type FlowItem = { key: string; label: string; value: number; color: string; hint?: string };

function FlowBlock({ title, items, scale, income }: { title: string; items: FlowItem[]; scale: number; income: number }) {
  const visible = items.filter((item) => item.value > 0.5);
  const total = visible.reduce((sum, item) => sum + item.value, 0);
  return (
    <View style={styles.flow}>
      <View style={styles.sectionHead}>
        <AppText weight="semibold" style={styles.grow}>
          {title}
        </AppText>
        <AppText weight="semibold" tabular>
          {formatCurrency(total)}
        </AppText>
      </View>
      <StackBar segments={visible} scale={scale} />
      {visible.map((item) => (
        <View key={item.key} style={styles.flowRow}>
          <View style={[styles.dot, { backgroundColor: item.color }]} />
          <View style={styles.grow}>
            <AppText variant="caption">{item.label}</AppText>
            {item.hint ? (
              <AppText variant="caption" tone="faint">
                {item.hint}
              </AppText>
            ) : null}
          </View>
          <AppText variant="caption" weight="semibold" tabular>
            {formatCurrency(item.value)}
          </AppText>
          <AppText variant="caption" tone="faint" tabular style={styles.share}>
            {formatPercent(shareOfIncome(item.value, income))}
          </AppText>
        </View>
      ))}
    </View>
  );
}

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
      `You spent ${share} of what you earned and kept ${formatPercent(shareOfIncome(summary.income - totalSpent, summary.income))}.`
    );
  } else {
    const funders = [
      summary.withdrawn > 0 && `${formatCurrency(summary.withdrawn)} taken out of investments`,
      summary.borrowed > 0 && `${formatCurrency(summary.borrowed)} borrowed`
    ].filter(Boolean);
    parts.push(
      `You spent ${share} of what you earned, ${formatCurrency(totalSpent - summary.income)} more than came in` +
        (funders.length > 0 ? `, covered by ${funders.join(' and ')}.` : '.')
    );
  }
  if (summary.oneOff > 0) {
    parts.push(
      `Leaving out ${formatCurrency(summary.oneOff)} of one-off spending, everyday spending was ${formatPercent(shareOfIncome(summary.spending, summary.income))} of income.`
    );
  }
  return parts.join(' ');
}

export function OverviewReport() {
  const { summary, loading } = useCashFlowSummary();
  const [selected, setSelected] = useState<number | null>(null);
  const [everydayOnly, setEverydayOnly] = useState(false);

  if (!summary) return loading ? <LoadingStateView label="Crunching the numbers" /> : <Empty>No data in this period</Empty>;

  const totalSpent = summary.spending + summary.oneOff;
  const monthCount = Math.max(summary.months.length, 1);
  const leftOver = summary.income - totalSpent;
  const netInvested = summary.invested - summary.withdrawn;
  const hasOneOff = summary.oneOff > 0;

  const sources: FlowItem[] = [
    { key: 'income', label: 'Earned income', value: summary.income, color: chartColors.income },
    {
      key: 'withdrawn',
      label: 'Taken out of investments',
      hint: 'Sold funds, stocks, FDs and other asset accounts',
      value: summary.withdrawn,
      color: chartColors.invested
    },
    { key: 'borrowed', label: 'Borrowed', value: summary.borrowed, color: chartColors.debt },
    {
      key: 'cash',
      label: 'From account balances',
      hint: 'Spent out of money you already had',
      value: Math.max(-summary.cashChange, 0),
      color: chartColors.cash
    }
  ];
  const uses: FlowItem[] = [
    { key: 'spending', label: 'Spending', value: summary.spending, color: chartColors.spending },
    { key: 'oneOff', label: 'One-off spending', value: summary.oneOff, color: chartColors.oneOff },
    {
      key: 'invested',
      label: 'Saved & invested',
      hint: summary.savedInGroups > 0 ? `Includes ${formatCurrency(summary.savedInGroups)} from saving groups` : undefined,
      value: summary.invested,
      color: chartColors.invested
    },
    { key: 'debt', label: 'Debt payments', value: summary.debtPaid, color: chartColors.debt },
    { key: 'kept', label: 'Kept in accounts', value: Math.max(summary.cashChange, 0), color: chartColors.cash }
  ];
  const scale = Math.max(
    sources.reduce((sum, s) => sum + Math.max(s.value, 0), 0),
    uses.reduce((sum, s) => sum + Math.max(s.value, 0), 0),
    1
  );

  const series: ChartSeries[] = [
    { key: 'income', label: 'Earned income', color: chartColors.income },
    { key: 'out', label: everydayOnly || !hasOneOff ? 'Spending' : 'Spending + one-off', color: chartColors.spending }
  ];
  const columns = summary.months.map((month) => ({
    key: month,
    label: monthLabel(month, summary.months.length <= 12 ? false : true),
    values: {
      income: summary.incomeMonthly[month] ?? 0,
      // a net-refund month shows as zero spending rather than a negative bar
      out:
        Math.max(summary.spendingMonthly[month] ?? 0, 0) +
        (everydayOnly ? 0 : Math.max(summary.oneOffMonthly[month] ?? 0, 0))
    }
  }));
  const pick = selected != null ? summary.months[selected] : null;
  const hasTransfers = summary.assetAccounts.length > 0 || summary.debtAccounts.length > 0;

  return (
    <View style={styles.stack}>
      <View style={styles.stats}>
        <Stat
          label="Earned income"
          color={chartColors.income}
          value={formatCurrency(summary.income)}
          detail={`${formatCurrency(summary.income / monthCount)} a month`}
        />
        <Stat
          label="Spent"
          color={chartColors.spending}
          value={formatCurrency(totalSpent)}
          detail={`${formatPercent(shareOfIncome(totalSpent, summary.income))} of income${
            hasOneOff ? ` · ${formatPercent(shareOfIncome(summary.spending, summary.income))} without one-offs` : ''
          }`}
        />
        <Stat
          label={leftOver >= 0 ? 'Left over' : 'Spent beyond income'}
          value={formatCurrency(Math.abs(leftOver))}
          tone={leftOver >= 0 ? undefined : 'danger'}
          detail={
            summary.income > 0
              ? `Savings rate ${formatPercent(shareOfIncome(leftOver, summary.income))}`
              : 'No earned income in this period'
          }
        />
        <Stat
          label={netInvested >= 0 ? 'Net invested' : 'Net withdrawn'}
          color={chartColors.invested}
          value={formatCurrency(Math.abs(netInvested))}
          detail={`${formatCurrency(summary.invested)} in · ${formatCurrency(summary.withdrawn)} out`}
        />
      </View>

      <Section title="Where the money came from and went">
        <AppText muted>{insight(summary)}</AppText>
        <FlowBlock title="Money in" items={sources} scale={scale} income={summary.income} />
        <FlowBlock title="Money out" items={uses} scale={scale} income={summary.income} />
        <AppText variant="caption" tone="faint">
          Percentages are of earned income.
        </AppText>
      </Section>

      <Section
        title="Month by month"
        right={
          hasOneOff ? (
            <Segmented
              options={[
                { value: 'all', label: 'All' },
                { value: 'everyday', label: 'Everyday' }
              ]}
              value={everydayOnly ? 'everyday' : 'all'}
              onChange={(value) => setEverydayOnly(value === 'everyday')}
            />
          ) : null
        }
      >
        <Legend items={series.map((s) => ({ label: s.label, color: s.color }))} />
        <BarChart columns={columns} series={series} mode="grouped" selected={selected} onSelect={toggle(setSelected)} />
        {pick ? (
          <MonthDetail
            title={monthLabel(pick)}
            rows={[
              { label: 'Earned income', color: chartColors.income, value: formatCurrency(summary.incomeMonthly[pick] ?? 0) },
              { label: 'Spending', color: chartColors.spending, value: formatCurrency(summary.spendingMonthly[pick] ?? 0) },
              ...(hasOneOff
                ? [{ label: 'One-off', color: chartColors.oneOff, value: formatCurrency(summary.oneOffMonthly[pick] ?? 0) }]
                : []),
              {
                label: 'Net invested',
                color: chartColors.invested,
                value: formatSignedCurrency((summary.investedMonthly[pick] ?? 0) - (summary.withdrawnMonthly[pick] ?? 0))
              }
            ]}
            footer={`Spent ${formatPercent(
              shareOfIncome(
                (summary.spendingMonthly[pick] ?? 0) + (summary.oneOffMonthly[pick] ?? 0),
                summary.incomeMonthly[pick] ?? 0
              )
            )} of income`}
          />
        ) : (
          <AppText variant="caption" tone="faint">
            Tap a month for its numbers.
          </AppText>
        )}
      </Section>

      {hasTransfers || summary.opening > 0 ? (
        <Note>
          {hasTransfers ? (
            <AppText variant="caption" muted>
              Money moved to or from asset, loan and liability accounts is never counted as income or spending,
              whichever category it was filed under.
            </AppText>
          ) : null}
          {summary.recategorizedCount > 0 ? (
            <AppText variant="caption" muted>
              {summary.recategorizedCount === 1 ? 'One transfer' : `${summary.recategorizedCount} transfers`} out of
              asset accounts ({formatCurrency(summary.recategorizedAmount)}) went straight to a category; they count as
              withdrawals here too. Nothing to clean up.
            </AppText>
          ) : null}
          {summary.opening > 0 ? (
            <AppText variant="caption" muted>
              Opening balances ({formatCurrency(summary.opening)}) aren't counted as income.
            </AppText>
          ) : null}
        </Note>
      ) : null}
    </View>
  );
}

// ── Spending ──

const TREATMENTS: { value: GroupTreatment; label: string }[] = [
  { value: 'spending', label: 'Spending' },
  { value: 'oneOff', label: 'One-off' },
  { value: 'saving', label: 'Saving' }
];

/** Top spending groups keep a fixed series color; the rest fold into Other. */
function colorGroups(groups: GroupSummary[]): Map<string, string> {
  const map = new Map<string, string>();
  const spent = groups.filter((group) => group.treatment !== 'saving' && group.total > 0);
  const named = spent.length > chartSeries.length ? chartSeries.length - 1 : chartSeries.length;
  spent.forEach((group, i) => map.set(group.id, i < named ? chartSeries[i] : chartColors.other));
  return map;
}

function stackedColumns(months: string[], lines: { key: string; monthly: Monthly }[]) {
  return months.map((month) => ({
    key: month,
    label: monthLabel(month, months.length > 12),
    values: Object.fromEntries(lines.map((line) => [line.key, Math.max(line.monthly[month] ?? 0, 0)]))
  }));
}

export function SpendingReport() {
  const dispatch = useAppDispatch();
  const { summary, loading } = useCashFlowSummary();
  const [expanded, setExpanded] = useState<string | null>(null);
  const [selected, setSelected] = useState<number | null>(null);

  if (!summary) return loading ? <LoadingStateView label="Loading spending" /> : <Empty>No data in this period</Empty>;
  if (!summary.groups.length) return <Empty>No spending in this period</Empty>;

  const groupColors = colorGroups(summary.groups);
  const monthCount = Math.max(summary.months.length, 1);
  const spent = summary.groups.filter((group) => group.treatment !== 'saving');
  const saving = summary.groups.filter((group) => group.treatment === 'saving');
  // scale bars to everyday groups so one huge one-off doesn't flatten the rest
  const everyday = summary.groups.filter((group) => group.treatment === 'spending');
  const max = Math.max(...(everyday.length ? everyday : summary.groups).map((group) => group.total), 1);

  const named = spent.filter((group) => groupColors.has(group.id) && groupColors.get(group.id) !== chartColors.other);
  const rest = spent.filter((group) => !named.includes(group));
  const otherMonthly: Monthly = {};
  for (const group of rest) {
    for (const [month, value] of Object.entries(group.monthly)) otherMonthly[month] = (otherMonthly[month] ?? 0) + value;
  }
  const series: ChartSeries[] = [
    ...named.map((group) => ({ key: group.id, label: group.name, color: groupColors.get(group.id) as string })),
    ...(rest.length ? [{ key: '__other__', label: `Other (${rest.length})`, color: chartColors.other }] : [])
  ];
  const columns = stackedColumns(summary.months, [
    ...named.map((group) => ({ key: group.id, monthly: group.monthly })),
    ...(rest.length ? [{ key: '__other__', monthly: otherMonthly }] : [])
  ]);
  const pick = selected != null ? columns[selected] : null;

  const changeTreatment = (group: GroupSummary, treatment: GroupTreatment) =>
    dispatch(
      setGroupTreatment({
        groupId: group.id,
        // picking the name-based guess drops the override
        treatment: treatment === defaultTreatment(group.id, group.name) ? null : treatment
      })
    );

  const renderGroup = (group: GroupSummary) => {
    const open = expanded === group.id;
    const color = group.treatment === 'saving' ? chartColors.invested : groupColors.get(group.id) ?? chartColors.other;
    return (
      <View key={group.id} style={styles.groupBlock}>
        <Pressable style={styles.groupRow} onPress={() => setExpanded(open ? null : group.id)}>
          {open ? <ChevronDown size={15} color={colors.faint} /> : <ChevronRight size={15} color={colors.faint} />}
          <View style={[styles.dot, { backgroundColor: color }]} />
          <View style={styles.grow}>
            <View style={styles.sectionHead}>
              <AppText weight="medium" numberOfLines={1} style={styles.grow}>
                {group.name}
              </AppText>
              <AppText weight="semibold" tabular>
                {formatSignedCurrency(group.total)}
              </AppText>
            </View>
            <RowBar value={group.total} max={max} color={color} />
            <AppText variant="caption" tone="faint">
              {formatPercent(shareOfIncome(group.total, summary.income))} of income ·{' '}
              {formatSignedCurrency(group.total / monthCount)}/mo
              {group.treatment === 'oneOff' ? ' · one-off' : ''}
            </AppText>
          </View>
        </Pressable>
        {open ? (
          <View style={styles.groupBody}>
            <View style={styles.treatmentRow}>
              <AppText variant="caption" muted>
                Counts as
              </AppText>
              <Segmented options={TREATMENTS} value={group.treatment} onChange={(value) => changeTreatment(group, value)} />
            </View>
            {group.categories.map((category) => (
              <View key={category.id} style={styles.categoryRow}>
                <AppText variant="caption" style={styles.grow} numberOfLines={1}>
                  {category.name}
                </AppText>
                <AppText variant="caption" tone="faint" tabular>
                  {formatSignedCurrency(category.total / monthCount)}/mo
                </AppText>
                <AppText variant="caption" weight="semibold" tabular style={styles.amountCol}>
                  {formatSignedCurrency(category.total)}
                </AppText>
              </View>
            ))}
          </View>
        ) : null}
      </View>
    );
  };

  return (
    <View style={styles.stack}>
      <Section title="Spending by group">
        <AppText variant="caption" muted>
          Mark groups that won't recur (a house, a wedding) as One-off to see your everyday spend rate, and groups that
          are really money set aside as Saving to count them with investments. Tap a group to change it.
        </AppText>
        {spent.map(renderGroup)}
        {saving.length ? (
          <>
            <AppText variant="label" tone="faint" style={styles.subhead}>
              Counted as saving
            </AppText>
            {saving.map(renderGroup)}
          </>
        ) : null}
      </Section>

      <Section title="Spending trend">
        <Legend items={series.map((s) => ({ label: s.label, color: s.color }))} />
        <BarChart columns={columns} series={series} height={220} selected={selected} onSelect={toggle(setSelected)} />
        {pick ? (
          <MonthDetail
            title={monthLabel(pick.key)}
            rows={series
              .filter((s) => (pick.values[s.key] ?? 0) > 0)
              .reverse()
              .map((s) => ({ label: s.label, color: s.color, value: formatCurrency(pick.values[s.key] ?? 0) }))}
            footer={`Total ${formatCurrency(Object.values(pick.values).reduce((sum, v) => sum + v, 0))}`}
          />
        ) : null}
      </Section>
    </View>
  );
}

// ── Income ──

const MAX_SOURCES = 5;

export function IncomeReport() {
  const { summary, loading } = useCashFlowSummary();
  const [selected, setSelected] = useState<number | null>(null);

  if (!summary) return loading ? <LoadingStateView label="Loading income" /> : <Empty>No data in this period</Empty>;

  const sources = summary.incomeSources;
  const named = sources.slice(0, sources.length > MAX_SOURCES ? MAX_SOURCES - 1 : MAX_SOURCES);
  const rest = sources.slice(named.length);
  const colorOf = (id: string) => {
    const i = named.findIndex((source) => source.id === id);
    return i === -1 ? chartColors.other : chartSeries[i];
  };
  const otherMonthly: Monthly = {};
  for (const source of rest) {
    for (const [month, value] of Object.entries(source.monthly)) otherMonthly[month] = (otherMonthly[month] ?? 0) + value;
  }
  const series: ChartSeries[] = [
    ...named.map((source) => ({ key: source.id, label: source.name, color: colorOf(source.id) })),
    ...(rest.length ? [{ key: '__other__', label: `Other (${rest.length})`, color: chartColors.other }] : [])
  ];
  const columns = stackedColumns(summary.months, [
    ...named.map((source) => ({ key: source.id, monthly: source.monthly })),
    ...(rest.length ? [{ key: '__other__', monthly: otherMonthly }] : [])
  ]);
  const pick = selected != null ? columns[selected] : null;
  const monthCount = Math.max(summary.months.length, 1);
  const max = Math.max(...sources.map((source) => source.total), 1);

  const withdrawals = summary.assetAccounts.filter((account) => account.in > 0.5);
  const borrowing = summary.debtAccounts.filter((account) => account.in > 0.5);

  return (
    <View style={styles.stack}>
      {sources.length ? (
        <>
          <Section title="Income sources">
            {sources.map((source) => (
              <View key={source.id} style={styles.sourceRow}>
                <View style={styles.sectionHead}>
                  <View style={[styles.dot, { backgroundColor: colorOf(source.id) }]} />
                  <AppText weight="medium" numberOfLines={1} style={styles.grow}>
                    {source.name}
                  </AppText>
                  <AppText weight="semibold" tabular>
                    {formatCurrency(source.total)}
                  </AppText>
                </View>
                <RowBar value={source.total} max={max} color={colorOf(source.id)} />
                <AppText variant="caption" tone="faint">
                  {formatPercent((source.total / (summary.income || 1)) * 100)} of income ·{' '}
                  {formatCurrency(source.total / monthCount)}/mo · paid {source.activeMonths} of {summary.months.length}{' '}
                  months
                </AppText>
              </View>
            ))}
          </Section>
          <Section title="Income by month">
            <Legend items={series.map((s) => ({ label: s.label, color: s.color }))} />
            <BarChart columns={columns} series={series} height={200} selected={selected} onSelect={toggle(setSelected)} />
            {pick ? (
              <MonthDetail
                title={monthLabel(pick.key)}
                rows={series
                  .filter((s) => (pick.values[s.key] ?? 0) > 0)
                  .map((s) => ({ label: s.label, color: s.color, value: formatCurrency(pick.values[s.key] ?? 0) }))}
                footer={`Total ${formatCurrency(summary.incomeMonthly[pick.key] ?? 0)}`}
              />
            ) : null}
          </Section>
        </>
      ) : (
        <Empty>No earned income in this period</Empty>
      )}

      {withdrawals.length || borrowing.length || summary.opening > 0 ? (
        <Section title="Came in, but not income">
          <AppText variant="caption" muted>
            This money landed in your budget accounts but wasn't earned, so it's left out of income and the spend rate.
          </AppText>
          {withdrawals.map((account) => (
            <View key={account.id} style={styles.detailRow}>
              <View style={[styles.dot, { backgroundColor: chartColors.invested }]} />
              <AppText variant="caption" style={styles.grow}>
                Taken out of {account.name}
              </AppText>
              <AppText variant="caption" weight="semibold" tabular>
                {formatCurrency(account.in)}
              </AppText>
            </View>
          ))}
          {borrowing.map((account) => (
            <View key={account.id} style={styles.detailRow}>
              <View style={[styles.dot, { backgroundColor: chartColors.debt }]} />
              <AppText variant="caption" style={styles.grow}>
                Borrowed from {account.name}
              </AppText>
              <AppText variant="caption" weight="semibold" tabular>
                {formatCurrency(account.in)}
              </AppText>
            </View>
          ))}
          {summary.opening > 0 ? (
            <View style={styles.detailRow}>
              <View style={[styles.dot, { backgroundColor: chartColors.cash }]} />
              <AppText variant="caption" style={styles.grow}>
                Opening balances
              </AppText>
              <AppText variant="caption" weight="semibold" tabular>
                {formatCurrency(summary.opening)}
              </AppText>
            </View>
          ) : null}
        </Section>
      ) : null}
    </View>
  );
}

// ── Investing ──

function FlowAccountRow({ name, put, taken, hint, closed }: { name: string; put: number; taken: number; hint?: string; closed?: boolean }) {
  return (
    <View style={styles.sourceRow}>
      <View style={styles.sectionHead}>
        <AppText weight="medium" numberOfLines={1} style={styles.grow}>
          {name}
          {closed ? <AppText variant="caption" tone="faint"> · closed</AppText> : null}
        </AppText>
        <AppText weight="semibold" tabular>
          {formatSignedCurrency(put - taken, true)}
        </AppText>
      </View>
      <AppText variant="caption" tone="faint">
        {formatCurrency(put)} in · {formatCurrency(taken)} out{hint ? ` · ${hint}` : ''}
      </AppText>
    </View>
  );
}

export function InvestingReport({ onOpenLoans }: { onOpenLoans?: () => void }) {
  const { summary, loading } = useCashFlowSummary();
  const [selected, setSelected] = useState<number | null>(null);

  if (!summary) return loading ? <LoadingStateView label="Loading investments" /> : <Empty>No data in this period</Empty>;
  if (summary.invested <= 0 && summary.withdrawn <= 0 && !summary.debtAccounts.length) {
    return (
      <Empty>
        No money moved to or from asset accounts in this period. Transfers to an asset account (mutual funds, stocks, an
        FD) show up here.
      </Empty>
    );
  }

  const net = summary.invested - summary.withdrawn;
  const columns = summary.months.map((month) => ({
    key: month,
    label: monthLabel(month, summary.months.length > 12),
    values: { net: (summary.investedMonthly[month] ?? 0) - (summary.withdrawnMonthly[month] ?? 0) }
  }));
  const pick = selected != null ? summary.months[selected] : null;
  const savingGroups = summary.groups.filter((group) => group.treatment === 'saving');

  return (
    <View style={styles.stack}>
      <View style={styles.stats}>
        <Stat
          label="Put in"
          color={chartColors.invested}
          value={formatCurrency(summary.invested)}
          detail={summary.savedInGroups > 0 ? `Includes ${formatCurrency(summary.savedInGroups)} from saving groups` : 'Moved into asset accounts'}
        />
        <Stat label="Taken out" value={formatCurrency(summary.withdrawn)} detail="Moved back into budget accounts" />
        <Stat
          label={net >= 0 ? 'Net invested' : 'Net withdrawn'}
          value={formatCurrency(Math.abs(net))}
          tone={net >= 0 ? 'success' : 'danger'}
          detail={net >= 0 ? 'New money into investments' : 'More came out than went in'}
        />
      </View>

      <Section title="Net invested by month">
        <AppText variant="caption" muted>
          Above the line, more went in than came out. Below it, you drew on your investments.
        </AppText>
        <BarChart
          columns={columns}
          series={[{ key: 'net', label: 'Net invested', color: chartColors.invested }]}
          selected={selected}
          onSelect={toggle(setSelected)}
        />
        {pick ? (
          <MonthDetail
            title={monthLabel(pick)}
            rows={[
              { label: 'Put in', value: formatCurrency(summary.investedMonthly[pick] ?? 0) },
              { label: 'Taken out', value: formatCurrency(summary.withdrawnMonthly[pick] ?? 0) }
            ]}
            footer={`Net ${formatSignedCurrency((summary.investedMonthly[pick] ?? 0) - (summary.withdrawnMonthly[pick] ?? 0), true)}`}
          />
        ) : null}
      </Section>

      <Section title="By account">
        {summary.assetAccounts.map((account) => (
          <FlowAccountRow
            key={account.id}
            name={account.name}
            put={account.out}
            taken={account.in}
            closed={account.closed}
            hint={account.recategorizedCount > 0 ? `${account.recategorizedCount} filed under a category` : undefined}
          />
        ))}
        {savingGroups.map((group) => (
          <FlowAccountRow
            key={group.id}
            name={group.name}
            put={Math.max(group.total, 0)}
            taken={Math.max(-group.total, 0)}
            hint="saving group"
          />
        ))}
      </Section>

      {summary.debtAccounts.length ? (
        <Section
          title="Loans & debt"
          right={
            onOpenLoans ? (
              <Pressable onPress={onOpenLoans} hitSlop={8}>
                <AppText variant="caption" weight="semibold" tone="primary">
                  Loans
                </AppText>
              </Pressable>
            ) : null
          }
        >
          {summary.debtAccounts.map((account) => (
            <View key={account.id} style={styles.sourceRow}>
              <View style={styles.sectionHead}>
                <AppText weight="medium" numberOfLines={1} style={styles.grow}>
                  {account.name}
                </AppText>
                <AppText weight="semibold" tabular>
                  {formatSignedCurrency(account.out - account.in, true)}
                </AppText>
              </View>
              <AppText variant="caption" tone="faint">
                {formatCurrency(account.out)} paid · {formatCurrency(account.in)} borrowed
              </AppText>
            </View>
          ))}
        </Section>
      ) : null}
    </View>
  );
}

// ── Net worth ──

const NET_SERIES: ChartSeries[] = [
  { key: 'netWorth', label: 'Net worth', color: chartSeries[0] },
  { key: 'cash', label: 'Cash', color: chartSeries[2] },
  { key: 'investments', label: 'Investments', color: chartColors.invested },
  { key: 'debt', label: 'Debt', color: chartColors.debt }
];

export function NetWorthReport() {
  const report = useAppSelector((state) => state.reports.netWorth);
  const loading = useAppSelector((state) => state.reports.netWorthLoading);
  const [selected, setSelected] = useState<number | null>(null);

  if (loading === LoadingState.PENDING || loading === LoadingState.IDLE) return <LoadingStateView label="Loading net worth" />;
  if (!report || !report.months.length) return <Empty>No data in this period</Empty>;

  // drop leading months before any account existed ("All" starts in 2020)
  const firstActive = report.months.findIndex((p) => p.assets !== 0 || p.liabilities !== 0);
  const points = (firstActive === -1 ? report.months : report.months.slice(firstActive)).map((point) => ({
    key: point.month,
    label: monthLabel(point.month),
    values: {
      netWorth: point.netWorth,
      cash: point.assets - point.investments,
      investments: point.investments,
      debt: Math.abs(point.liabilities)
    },
    loans: Math.abs(point.loans ?? 0)
  }));
  const first = points[0];
  const latest = points[points.length - 1];
  const since = monthLabel(first.key);
  const pick = selected != null ? points[selected] : null;

  return (
    <View style={styles.stack}>
      <View style={styles.stats}>
        <Stat
          label="Net worth"
          color={NET_SERIES[0].color}
          value={formatCurrency(latest.values.netWorth)}
          detail={`${formatSignedCurrency(latest.values.netWorth - first.values.netWorth, true)} since ${since}`}
        />
        <Stat label="Cash" color={NET_SERIES[1].color} value={formatCurrency(latest.values.cash)} detail="Checking and savings" />
        <Stat
          label="Investments"
          color={chartColors.invested}
          value={formatCurrency(latest.values.investments)}
          detail={`${formatSignedCurrency(latest.values.investments - first.values.investments, true)} since ${since}`}
        />
        <Stat
          label="Debt"
          color={chartColors.debt}
          value={formatCurrency(latest.values.debt)}
          detail={
            latest.loans > 0
              ? `${formatCurrency(latest.loans)} in loans · ${formatCurrency(latest.values.debt - latest.loans)} other`
              : 'Cards, liabilities and loans'
          }
        />
      </View>

      <Section title="Net worth over time">
        <Legend items={NET_SERIES.map((s) => ({ label: s.label, color: s.color }))} />
        <LineChart points={points} series={NET_SERIES} height={220} selected={selected} onSelect={toggle(setSelected)} />
        {pick ? (
          <MonthDetail
            title={pick.label}
            rows={NET_SERIES.map((s) => ({ label: s.label, color: s.color, value: formatCurrency(pick.values[s.key as keyof typeof pick.values]) }))}
          />
        ) : (
          <AppText variant="caption" tone="faint">
            Tap the chart for a month's figures.
          </AppText>
        )}
        {latest.loans > 0 ? (
          <AppText variant="caption" tone="faint">
            Loan balances come from each loan's rate and recorded payments, so the interest part of every EMI isn't
            counted as repaid.
          </AppText>
        ) : null}
      </Section>
    </View>
  );
}

const styles = StyleSheet.create({
  stack: {
    gap: spacing.lg
  },
  stats: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sm
  },
  section: {
    gap: spacing.md
  },
  sectionHead: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  subhead: {
    marginTop: spacing.sm
  },
  grow: {
    flex: 1,
    minWidth: 0
  },
  center: {
    textAlign: 'center'
  },
  empty: {
    paddingVertical: spacing.xxl
  },
  note: {
    flexDirection: 'row',
    gap: spacing.sm,
    padding: spacing.lg,
    borderRadius: radii.lg,
    backgroundColor: colors.surface
  },
  detail: {
    gap: spacing.xs,
    padding: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong
  },
  detailRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  dot: {
    width: 9,
    height: 9,
    borderRadius: 3
  },
  flow: {
    gap: spacing.sm
  },
  flowRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  share: {
    width: 42,
    textAlign: 'right'
  },
  groupBlock: {
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    overflow: 'hidden'
  },
  groupRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    padding: spacing.md
  },
  groupBody: {
    gap: spacing.sm,
    paddingHorizontal: spacing.md,
    paddingBottom: spacing.md
  },
  treatmentRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.sm
  },
  categoryRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingLeft: spacing.xl
  },
  amountCol: {
    minWidth: 72,
    textAlign: 'right'
  },
  sourceRow: {
    gap: spacing.xs
  }
});
