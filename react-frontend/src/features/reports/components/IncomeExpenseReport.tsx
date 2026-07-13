import { Fragment, useState } from 'react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { CaretDown, CaretRight } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { formatCompactCurrency, formatCurrency } from '@/features/dashboard/utils';
import { getSelectedMonthInHumanFormat } from '@/utils/date.utils';
import { selectIncomeExpenseLoading, selectIncomeExpenseReport } from '../store/reportSlice';
import styles from './IncomeExpenseReport.module.css';

// Same cash-flow colors as the dashboard SpendingTrends chart; hex because
// SVG fills can't resolve CSS vars.
const INCOME_COLOR = '#1baf7a';
const EXPENSE_COLOR = '#3987e5';
const AXIS_COLOR = '#a9a9a9';
const GRID_COLOR = 'rgba(255, 255, 255, 0.07)';

function monthLabel(monthKey: string): string {
  const [year, month] = monthKey.split('-');
  const date = new Date(parseInt(year, 10), parseInt(month, 10) - 1, 1);
  return `${date.toLocaleString('en-us', { month: 'short' })} ${year.slice(2)}`;
}

interface TooltipEntry {
  dataKey?: string | number;
  name?: string | number;
  value?: string | number;
  color?: string;
}

interface ChartTooltipProps {
  active?: boolean;
  payload?: TooltipEntry[];
  label?: string | number;
}

const ChartTooltip = ({ active, payload, label }: ChartTooltipProps) => {
  if (!active || !payload || payload.length === 0) return null;
  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipLabel}>{label}</div>
      {payload.map((entry) => (
        <div key={String(entry.dataKey)} className={styles.tooltipRow}>
          <span className={styles.tooltipSwatch} style={{ background: entry.color }} />
          <span className={styles.tooltipName}>{entry.name}</span>
          <span className={styles.tooltipValue}>{formatCurrency(Number(entry.value ?? 0))}</span>
        </div>
      ))}
    </div>
  );
};

export default function IncomeExpenseReport() {
  const report = useAppSelector(selectIncomeExpenseReport);
  const loading = useAppSelector(selectIncomeExpenseLoading);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());

  if (loading === LoadingState.PENDING || loading === LoadingState.IDLE) {
    return <div className={styles.emptyState}>Loading income vs expense report…</div>;
  }
  if (!report || report.months.length === 0) {
    return <div className={styles.emptyState}>No data in the selected period</div>;
  }

  const chartData = report.months.map((month) => ({
    monthLabel: monthLabel(month),
    income: report.income.totals[month] ?? 0,
    expense: report.expense.totals[month] ?? 0,
  }));
  const hasData = chartData.some((d) => d.income !== 0 || d.expense !== 0);
  const rangeLabel = `${getSelectedMonthInHumanFormat(report.startMonth)} – ${getSelectedMonthInHumanFormat(report.endMonth)}`;

  const toggleGroup = (groupId: string) => {
    setCollapsedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(groupId)) {
        next.delete(groupId);
      } else {
        next.add(groupId);
      }
      return next;
    });
  };

  const sumAmounts = (amounts: Record<string, number>) =>
    report.months.reduce((sum, month) => sum + (amounts[month] ?? 0), 0);

  return (
    <div className={styles.wrapper}>
      <div className={styles.card}>
        <div className={styles.cardHeader}>
          <h2 className={styles.cardTitle}>Income vs Expense</h2>
          <div className={styles.legend}>
            <span className={styles.legendItem}>
              <span className={styles.legendDot} style={{ background: INCOME_COLOR }} />
              Income
            </span>
            <span className={styles.legendItem}>
              <span className={styles.legendDot} style={{ background: EXPENSE_COLOR }} />
              Expense
            </span>
          </div>
          <span className={styles.rangeLabel}>{rangeLabel}</span>
        </div>

        {hasData ? (
          <ResponsiveContainer width="100%" height={260} initialDimension={{ width: 600, height: 260 }}>
            <BarChart data={chartData} barGap={2} barCategoryGap="28%">
              <CartesianGrid vertical={false} stroke={GRID_COLOR} />
              <XAxis
                dataKey="monthLabel"
                axisLine={{ stroke: GRID_COLOR }}
                tickLine={false}
                tick={{ fill: AXIS_COLOR, fontSize: 13.5 }}
              />
              <YAxis
                axisLine={false}
                tickLine={false}
                width={60}
                tick={{ fill: AXIS_COLOR, fontSize: 13.5 }}
                tickFormatter={(value: number) => formatCompactCurrency(value)}
              />
              <Tooltip content={<ChartTooltip />} cursor={{ fill: 'rgba(255, 255, 255, 0.04)' }} />
              <Bar
                dataKey="income"
                name="Income"
                fill={INCOME_COLOR}
                radius={[4, 4, 0, 0]}
                maxBarSize={20}
                isAnimationActive={false}
              />
              <Bar
                dataKey="expense"
                name="Expense"
                fill={EXPENSE_COLOR}
                radius={[4, 4, 0, 0]}
                maxBarSize={20}
                isAnimationActive={false}
              />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <div className={styles.emptyState}>No transactions in the selected period</div>
        )}
      </div>

      <div className={styles.card}>
        <div className={styles.tableScroll}>
          <table className={styles.matrix}>
            <thead>
              <tr>
                <th className={styles.nameCol}></th>
                {report.months.map((month) => (
                  <th key={month} className={styles.amountCol}>
                    {monthLabel(month)}
                  </th>
                ))}
                <th className={styles.amountCol}>Total</th>
              </tr>
            </thead>
            <tbody>
              <tr className={styles.sectionRow}>
                <td className={styles.nameCol}>Income</td>
                {report.months.map((month) => (
                  <td key={month} className={styles.amountCol}>
                    {formatCurrency(report.income.totals[month] ?? 0)}
                  </td>
                ))}
                <td className={styles.amountCol}>{formatCurrency(sumAmounts(report.income.totals))}</td>
              </tr>
              {report.income.payees.map((payee) => (
                <tr key={payee.payeeId} className={styles.detailRow}>
                  <td className={styles.nameCol}>{payee.name || 'No payee'}</td>
                  {report.months.map((month) => (
                    <td key={month} className={styles.amountCol}>
                      {formatCurrency(payee.amounts[month] ?? 0)}
                    </td>
                  ))}
                  <td className={styles.amountCol}>{formatCurrency(sumAmounts(payee.amounts))}</td>
                </tr>
              ))}

              <tr className={styles.sectionRow}>
                <td className={styles.nameCol}>Expense</td>
                {report.months.map((month) => (
                  <td key={month} className={styles.amountCol}>
                    {formatCurrency(report.expense.totals[month] ?? 0)}
                  </td>
                ))}
                <td className={styles.amountCol}>{formatCurrency(sumAmounts(report.expense.totals))}</td>
              </tr>
              {report.expense.groups.map((group) => {
                const collapsed = collapsedGroups.has(group.categoryGroupId);
                return (
                  <Fragment key={group.categoryGroupId}>
                    <tr className={styles.groupRow}>
                      <td className={styles.nameCol}>
                        <button
                          className={styles.groupToggle}
                          onClick={() => toggleGroup(group.categoryGroupId)}>
                          {collapsed ? <CaretRight size={14} /> : <CaretDown size={14} />}
                          {group.name}
                        </button>
                      </td>
                      {report.months.map((month) => (
                        <td key={month} className={styles.amountCol}>
                          {formatCurrency(group.totals[month] ?? 0)}
                        </td>
                      ))}
                      <td className={styles.amountCol}>{formatCurrency(sumAmounts(group.totals))}</td>
                    </tr>
                    {!collapsed &&
                      group.categories.map((category) => (
                        <tr key={category.categoryId} className={styles.detailRow}>
                          <td className={styles.nameColIndent}>{category.name}</td>
                          {report.months.map((month) => (
                            <td key={month} className={styles.amountCol}>
                              {formatCurrency(category.amounts[month] ?? 0)}
                            </td>
                          ))}
                          <td className={styles.amountCol}>{formatCurrency(sumAmounts(category.amounts))}</td>
                        </tr>
                      ))}
                  </Fragment>
                );
              })}

              <tr className={styles.netRow}>
                <td className={styles.nameCol}>Net</td>
                {report.months.map((month) => {
                  const net = report.net[month] ?? 0;
                  return (
                    <td
                      key={month}
                      className={net < 0 ? styles.amountColNegative : styles.amountCol}>
                      {formatCurrency(net)}
                    </td>
                  );
                })}
                <td className={styles.amountCol}>{formatCurrency(sumAmounts(report.net))}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
