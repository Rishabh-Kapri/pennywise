import { useId, useMemo } from 'react';
import { Area, AreaChart, CartesianGrid, ReferenceDot, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import type { BalancePoint } from '../../types';
import {
  formatDayMonth,
  formatSignedCurrency,
  formatTickCurrency,
  formatWeekdayDate,
  niceTicks,
} from '../../utils';
import styles from './BalanceCard.module.css';

// SVG presentation colors follow the shared theme.
const LINE_COLOR = 'var(--accent)';
const AXIS_COLOR = 'var(--muted)';
const GRID_COLOR = 'var(--line)';
const SURFACE_COLOR = 'var(--dash-surface)';
const CHART_HEIGHT = 210;

interface TooltipEntry {
  payload?: BalancePoint;
}

interface BalanceTooltipProps {
  active?: boolean;
  payload?: TooltipEntry[];
}

const BalanceTooltip = ({ active, payload }: BalanceTooltipProps) => {
  const point = payload?.[0]?.payload;
  if (!active || !point) return null;

  return (
    <div className={styles.tooltip}>
      <span className={styles.tooltipDate}>{formatWeekdayDate(point.date)}</span>
      <span className={styles.tooltipValue}>{formatSignedCurrency(point.balance)}</span>
      {Math.round(point.change) !== 0 && (
        <span className={styles.tooltipChange}>{formatSignedCurrency(point.change, true)} that day</span>
      )}
    </div>
  );
};

interface BalanceChartProps {
  data: BalancePoint[];
}

/** End-of-day balance as a stepped line: it only moves when money does. */
export default function BalanceChart({ data }: BalanceChartProps) {
  const gradientId = `balance-fill-${useId().replace(/:/g, '')}`;

  const ticks = useMemo(() => {
    if (data.length === 0) return [0, 1];
    const values = data.map((point) => point.balance);
    return niceTicks(Math.min(...values), Math.max(...values));
  }, [data]);
  const domain: [number, number] = [ticks[0], ticks[ticks.length - 1]];
  const step = ticks[1] - ticks[0];

  if (data.length < 2) {
    return <div className={styles.chartEmpty}>Your balance history will show up here</div>;
  }

  const first = data[0];
  const last = data[data.length - 1];

  return (
    <figure className={styles.chart}>
      <figcaption className={styles.srOnly}>
        Balance went from {formatSignedCurrency(first.balance)} on {formatWeekdayDate(first.date)} to{' '}
        {formatSignedCurrency(last.balance)} today.
      </figcaption>
      <ResponsiveContainer width="100%" height={CHART_HEIGHT} initialDimension={{ width: 600, height: CHART_HEIGHT }}>
        <AreaChart data={data} margin={{ top: 8, right: 0, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={LINE_COLOR} stopOpacity={0.22} />
              <stop offset="100%" stopColor={LINE_COLOR} stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid vertical={false} stroke={GRID_COLOR} />
          <XAxis
            dataKey="date"
            tickFormatter={formatDayMonth}
            axisLine={false}
            tickLine={false}
            tickMargin={8}
            minTickGap={56}
            tick={{ fill: AXIS_COLOR, fontSize: 12.5 }}
          />
          <YAxis
            orientation="right"
            domain={domain}
            ticks={ticks}
            tickFormatter={(value: number) => formatTickCurrency(value, step)}
            axisLine={false}
            tickLine={false}
            width={54}
            tick={{ fill: AXIS_COLOR, fontSize: 12.5 }}
          />
          <Tooltip content={<BalanceTooltip />} cursor={{ stroke: 'var(--color-border)', strokeWidth: 1 }} />
          <Area
            type="stepAfter"
            dataKey="balance"
            stroke={LINE_COLOR}
            strokeWidth={2}
            fill={`url(#${gradientId})`}
            baseValue={domain[0]}
            isAnimationActive={false}
            activeDot={{ r: 4.5, fill: LINE_COLOR, stroke: SURFACE_COLOR, strokeWidth: 2 }}
          />
          <ReferenceDot x={last.date} y={last.balance} r={4} fill={LINE_COLOR} stroke={SURFACE_COLOR} strokeWidth={2} />
        </AreaChart>
      </ResponsiveContainer>
    </figure>
  );
}
