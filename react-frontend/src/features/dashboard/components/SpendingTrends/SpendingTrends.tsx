import { useAppSelector } from '@/app/hooks';
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { selectSpendingTrends } from '../../store/dashboardSlice';
import { ChartBar } from '@phosphor-icons/react';
import { formatCurrency, formatCompactCurrency } from '../../utils';
import styles from './SpendingTrends.module.css';

// Categorical series colors, CVD-validated against the card surface (#323232).
// Hex constants because SVG fill attributes don't resolve CSS custom properties.
const INCOME_COLOR = '#1baf7a';
const EXPENSE_COLOR = '#3987e5';
const AXIS_COLOR = '#a9a9a9';
const GRID_COLOR = 'rgba(255, 255, 255, 0.07)';

interface TooltipEntry {
  dataKey?: string | number;
  name?: string | number;
  value?: string | number;
  color?: string;
}

interface CashFlowTooltipProps {
  active?: boolean;
  payload?: TooltipEntry[];
  label?: string | number;
}

const CashFlowTooltip = ({ active, payload, label }: CashFlowTooltipProps) => {
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

export default function SpendingTrends() {
  const trends = useAppSelector(selectSpendingTrends);
  const hasData = trends.some((trend) => trend.income > 0 || trend.expenses > 0);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2 className={styles.title}>
          <ChartBar size={18} />
          Cash flow
        </h2>
        <div className={styles.legend}>
          <span className={styles.legendItem}>
            <span className={styles.legendDot} style={{ background: INCOME_COLOR }} />
            Income
          </span>
          <span className={styles.legendItem}>
            <span className={styles.legendDot} style={{ background: EXPENSE_COLOR }} />
            Spending
          </span>
        </div>
      </div>

      {hasData ? (
        <div className={styles.chartArea}>
          <ResponsiveContainer
            width="100%"
            height={240}
            initialDimension={{ width: 600, height: 240 }}
          >
            <BarChart data={trends} barGap={2} barCategoryGap="28%">
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
              <Tooltip
                content={<CashFlowTooltip />}
                cursor={{ fill: 'rgba(255, 255, 255, 0.04)' }}
              />
              <Bar
                dataKey="income"
                name="Income"
                fill={INCOME_COLOR}
                radius={[4, 4, 0, 0]}
                maxBarSize={20}
                isAnimationActive={false}
              />
              <Bar
                dataKey="expenses"
                name="Spending"
                fill={EXPENSE_COLOR}
                radius={[4, 4, 0, 0]}
                maxBarSize={20}
                isAnimationActive={false}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className={styles.emptyState}>No transactions in the last 6 months</div>
      )}
    </div>
  );
}
