import { Fragment, useState } from 'react';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { CaretDown, CaretRight, ChartBar, ListBullets } from '@phosphor-icons/react';
import { useAppDispatch } from '@/app/hooks';
import { DashboardCard, Segmented } from '@/features/dashboard/components/DashboardCard';
import { formatCurrency } from '@/features/dashboard/utils';
import {
  defaultTreatment,
  formatPercent,
  monthLabel,
  shareOfIncome,
  type CashFlowSummary,
  type GroupSummary,
} from '../cashFlow';
import { useCashFlowSummary } from '../hooks/useCashFlowSummary';
import { setGroupTreatment } from '../store/reportSlice';
import type { GroupTreatment } from '../types/report.types';
import { AXIS_TICK, COLORS, CURSOR_FILL, GRID_COLOR, SERIES, axisCurrency, formatMonthTick } from '../chartTheme';
import { EmptyState, MonthTable, RowBar, TooltipCard, type MonthTableRow } from './ReportParts';
import styles from './SpendingReport.module.css';

const TREATMENT_OPTIONS: { value: GroupTreatment; label: string }[] = [
  { value: 'spending', label: 'Spending' },
  { value: 'oneOff', label: 'One-off' },
  { value: 'saving', label: 'Saving' },
];

const VIEW_OPTIONS = [
  { value: 'chart', label: 'Chart' },
  { value: 'table', label: 'Table' },
] as const;

const OTHER_KEY = '__other__';

/** Top spending groups keep a fixed series color; the rest fold into Other. */
function colorGroups(groups: GroupSummary[]): Map<string, string> {
  const colors = new Map<string, string>();
  const spent = groups.filter((group) => group.treatment !== 'saving' && group.total > 0);
  const named = spent.length > SERIES.length ? SERIES.length - 1 : SERIES.length;
  spent.forEach((group, i) => colors.set(group.id, i < named ? SERIES[i] : COLORS.other));
  return colors;
}

export default function SpendingReport() {
  const { summary, loading } = useCashFlowSummary();

  if (!summary) {
    return <EmptyState>{loading ? 'Loading spending…' : 'No data in the selected period'}</EmptyState>;
  }
  if (summary.groups.length === 0) {
    return <EmptyState>No spending in the selected period</EmptyState>;
  }

  const colors = colorGroups(summary.groups);

  return (
    <div className={styles.stack}>
      <GroupsCard summary={summary} colors={colors} />
      <TrendCard summary={summary} colors={colors} />
    </div>
  );
}

function GroupsCard({ summary, colors }: { summary: CashFlowSummary; colors: Map<string, string> }) {
  const dispatch = useAppDispatch();
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const monthCount = Math.max(summary.months.length, 1);
  const spent = summary.groups.filter((group) => group.treatment !== 'saving');
  const saving = summary.groups.filter((group) => group.treatment === 'saving');
  // scale bars to everyday groups so one huge one-off doesn't flatten the rest
  const everyday = summary.groups.filter((group) => group.treatment === 'spending');
  const max = Math.max(...(everyday.length > 0 ? everyday : summary.groups).map((group) => group.total), 1);

  const toggle = (id: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const changeTreatment = (group: GroupSummary, treatment: GroupTreatment) =>
    dispatch(
      setGroupTreatment({
        groupId: group.id,
        // picking the name-based guess drops the override
        treatment: treatment === defaultTreatment(group.id, group.name) ? null : treatment,
      }),
    );

  const renderGroup = (group: GroupSummary) => {
    const open = expanded.has(group.id);
    const color = group.treatment === 'saving' ? COLORS.invested : (colors.get(group.id) ?? COLORS.other);
    return (
      <Fragment key={group.id}>
        <li className={styles.row} data-treatment={group.treatment}>
          <button
            type="button"
            className={styles.name}
            aria-expanded={open}
            onClick={() => toggle(group.id)}
            disabled={group.categories.length === 0}
          >
            {open ? <CaretDown size={13} /> : <CaretRight size={13} />}
            <span className={styles.dot} style={{ background: color }} />
            <span className={styles.nameText}>{group.name}</span>
            {group.treatment === 'oneOff' && <span className={styles.tag}>One-off</span>}
          </button>
          <select
            className={styles.treatment}
            value={group.treatment}
            aria-label={`Count ${group.name} as`}
            title={group.isDefault ? 'Guessed from the group name' : 'Your choice'}
            onChange={(e) => changeTreatment(group, e.target.value as GroupTreatment)}
          >
            {TREATMENT_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <RowBar value={group.total} max={max} color={color} />
          <span className={styles.amount}>{formatSigned(group.total)}</span>
          <span className={styles.share}>{formatPercent(shareOfIncome(group.total, summary.income))}</span>
          <span className={styles.average}>{formatSigned(group.total / monthCount)}</span>
        </li>
        {open &&
          group.categories.map((category) => (
            <li key={category.id} className={styles.subRow}>
              <span className={styles.subName}>{category.name}</span>
              <span />
              <RowBar value={category.total} max={max} color={color} />
              <span className={styles.amount}>{formatSigned(category.total)}</span>
              <span className={styles.share}>{formatPercent(shareOfIncome(category.total, summary.income))}</span>
              <span className={styles.average}>{formatSigned(category.total / monthCount)}</span>
            </li>
          ))}
      </Fragment>
    );
  };

  return (
    <DashboardCard title="Spending by group" icon={<ListBullets size={18} />}>
      <p className={styles.help}>
        Mark groups that won't recur (a house, a wedding) as <strong>One-off</strong> to see your everyday spend rate,
        and groups that are really money set aside as <strong>Saving</strong> to count them with your investments.
      </p>
      <ul className={styles.list}>
        <li className={styles.headRow} aria-hidden="true">
          <span>Group</span>
          <span>Counts as</span>
          <span />
          <span className={styles.amount}>Total</span>
          <span className={styles.share}>Of income</span>
          <span className={styles.average}>Per month</span>
        </li>
        {spent.map(renderGroup)}
        {saving.length > 0 && (
          <>
            <li className={styles.sectionRow}>Counted as saving</li>
            {saving.map(renderGroup)}
          </>
        )}
      </ul>
    </DashboardCard>
  );
}

function formatSigned(value: number) {
  const rounded = Math.round(value);
  return `${rounded < 0 ? '−' : ''}${formatCurrency(rounded)}`;
}

function TrendCard({ summary, colors }: { summary: CashFlowSummary; colors: Map<string, string> }) {
  const [view, setView] = useState<'chart' | 'table'>('chart');
  const spent = summary.groups.filter((group) => group.treatment !== 'saving');
  const named = spent.filter((group) => colors.get(group.id) !== COLORS.other && colors.has(group.id));
  const rest = spent.filter((group) => !named.includes(group));

  const data = summary.months.map((month) => {
    const row: Record<string, number | string> = { month };
    for (const group of named) row[group.id] = Math.max(group.monthly[month] ?? 0, 0);
    if (rest.length > 0) {
      row[OTHER_KEY] = Math.max(
        rest.reduce((sum, group) => sum + (group.monthly[month] ?? 0), 0),
        0,
      );
    }
    return row;
  });
  const series = [
    ...named.map((group) => ({ key: group.id, name: group.name, color: colors.get(group.id)! })),
    ...(rest.length > 0 ? [{ key: OTHER_KEY, name: `Other (${rest.length})`, color: COLORS.other }] : []),
  ];

  const tableRows: MonthTableRow[] = spent.flatMap((group) => [
    { key: group.id, label: group.name, monthly: group.monthly, emphasis: true },
    ...group.categories.map((category) => ({
      key: `${group.id}:${category.id}`,
      label: category.name,
      monthly: category.monthly,
      indent: true,
    })),
  ]);

  return (
    <DashboardCard
      title="Spending trend"
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
          <ResponsiveContainer width="100%" height={300} initialDimension={{ width: 600, height: 300 }}>
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
                  stackId="spend"
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
        <MonthTable months={summary.months} rows={tableRows} />
      )}
    </DashboardCard>
  );
}
