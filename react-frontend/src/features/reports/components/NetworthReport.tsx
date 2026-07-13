import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { formatCompactCurrency, formatCurrency } from '@/features/dashboard/utils';
import { getSelectedMonthInHumanFormat } from '@/utils/date.utils';
import { selectNetWorthLoading, selectNetWorthReport } from '../store/reportSlice';
import styles from './NetworthReport.module.css';

// CVD-validated against the card surface (#323232); hex because SVG
// fills can't resolve CSS vars.
const NET_WORTH_COLOR = '#3987e5';
const ASSETS_COLOR = '#1baf7a';
const DEBT_COLOR = '#d55b33';
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
  stroke?: string;
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
          <span
            className={styles.tooltipSwatch}
            style={{ background: entry.stroke ?? entry.color }}
          />
          <span className={styles.tooltipName}>{entry.name}</span>
          <span className={styles.tooltipValue}>{formatCurrency(Number(entry.value ?? 0))}</span>
        </div>
      ))}
    </div>
  );
};

export default function NetworthReport() {
  const report = useAppSelector(selectNetWorthReport);
  const loading = useAppSelector(selectNetWorthLoading);

  if (loading === LoadingState.PENDING || loading === LoadingState.IDLE) {
    return <div className={styles.emptyState}>Loading net worth report…</div>;
  }
  if (!report || report.months.length === 0) {
    return <div className={styles.emptyState}>No data in the selected period</div>;
  }

  const chartData = report.months.map((point) => ({
    monthLabel: monthLabel(point.month),
    netWorth: point.netWorth,
    assets: point.assets,
    debt: Math.abs(point.liabilities),
  }));
  const latest = report.months[report.months.length - 1];
  const rangeLabel = `${getSelectedMonthInHumanFormat(report.startMonth)} – ${getSelectedMonthInHumanFormat(report.endMonth)}`;

  return (
    <div className={styles.card}>
      <div className={styles.cardHeader}>
        <h2 className={styles.cardTitle}>Net worth over time</h2>
        <div className={styles.legend}>
          <span className={styles.legendItem}>
            <span className={styles.legendDot} style={{ background: NET_WORTH_COLOR }} />
            Net worth
          </span>
          <span className={styles.legendItem}>
            <span className={styles.legendDot} style={{ background: ASSETS_COLOR }} />
            Assets
          </span>
          <span className={styles.legendItem}>
            <span className={styles.legendDot} style={{ background: DEBT_COLOR }} />
            Debt
          </span>
        </div>
        <span className={styles.rangeLabel}>{rangeLabel}</span>
      </div>

      <div className={styles.stats}>
        <div className={styles.stat}>
          <span className={styles.statLabel}>Current net worth</span>
          <span className={styles.statValue}>{formatCurrency(latest.netWorth)}</span>
        </div>
        <div className={styles.stat}>
          <span className={styles.statLabel}>Assets</span>
          <span className={styles.statValue}>{formatCurrency(latest.assets)}</span>
        </div>
        <div className={styles.stat}>
          <span className={styles.statLabel}>Debt</span>
          <span className={styles.statValue}>{formatCurrency(Math.abs(latest.liabilities))}</span>
        </div>
      </div>

      <ResponsiveContainer width="100%" height={300} initialDimension={{ width: 600, height: 300 }}>
        <ComposedChart data={chartData}>
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
          <Tooltip content={<ChartTooltip />} cursor={{ stroke: 'rgba(255, 255, 255, 0.2)' }} />
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
          <Line
            dataKey="assets"
            name="Assets"
            stroke={ASSETS_COLOR}
            strokeWidth={2}
            dot={false}
            isAnimationActive={false}
          />
          <Line
            dataKey="debt"
            name="Debt"
            stroke={DEBT_COLOR}
            strokeWidth={2}
            dot={false}
            isAnimationActive={false}
          />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}
