import { useState } from 'react';
import { useAppSelector } from '@/app/hooks';
import { selectSpendingTrends } from '../../store/dashboardSlice';
import { BarChart3 } from 'lucide-react';
import styles from './SpendingTrends.module.css';

const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount);
};

interface TooltipData {
  month: string;
  income: number;
  expenses: number;
  x: number;
  y: number;
}

export default function SpendingTrends() {
  const trends = useAppSelector(selectSpendingTrends);
  const [tooltip, setTooltip] = useState<TooltipData | null>(null);

  if (trends.length === 0) {
    return (
      <div className={styles.container}>
        <div className={styles.header}>
          <h2 className={styles.title}>
            <BarChart3 size={20} className={styles.titleIcon} />
            Spending Trends
          </h2>
        </div>
        <div className={styles.emptyState}>No transaction data available</div>
      </div>
    );
  }

  // Chart dimensions
  const width = 100; // SVG viewBox width percentage
  const height = 100;
  const padding = { top: 10, bottom: 25, left: 5, right: 5 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = height - padding.top - padding.bottom;

  // Calculate max value for scaling
  const maxValue = Math.max(
    ...trends.flatMap((t) => [t.income, t.expenses]),
    1
  );

  // Bar dimensions
  const barGroupWidth = chartWidth / trends.length;
  const barWidth = barGroupWidth * 0.35;
  const barGap = barGroupWidth * 0.05;

  const handleMouseEnter = (
    trend: typeof trends[0],
    _index: number,
    event: React.MouseEvent
  ) => {
    const rect = event.currentTarget.getBoundingClientRect();
    const containerRect = (event.currentTarget.closest(`.${styles.chartContainer}`) as HTMLElement)?.getBoundingClientRect();
    
    if (containerRect) {
      setTooltip({
        month: trend.monthLabel,
        income: trend.income,
        expenses: trend.expenses,
        x: rect.left - containerRect.left + rect.width / 2,
        y: rect.top - containerRect.top - 10,
      });
    }
  };

  const handleMouseLeave = () => {
    setTooltip(null);
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2 className={styles.title}>
          <BarChart3 size={20} className={styles.titleIcon} />
          Spending Trends
        </h2>
        <div className={styles.legend}>
          <div className={styles.legendItem}>
            <div className={`${styles.legendDot} ${styles.income}`} />
            Income
          </div>
          <div className={styles.legendItem}>
            <div className={`${styles.legendDot} ${styles.expenses}`} />
            Expenses
          </div>
        </div>
      </div>

      <div className={styles.chartContainer}>
        <svg
          className={styles.chart}
          viewBox={`0 0 ${width} ${height}`}
          preserveAspectRatio="xMidYMid meet"
        >
          {/* Grid lines */}
          {[0, 25, 50, 75, 100].map((percent) => (
            <line
              key={percent}
              className={styles.gridLine}
              x1={padding.left}
              y1={padding.top + (chartHeight * (100 - percent)) / 100}
              x2={width - padding.right}
              y2={padding.top + (chartHeight * (100 - percent)) / 100}
            />
          ))}

          {/* Bars */}
          {trends.map((trend, index) => {
            const groupX = padding.left + index * barGroupWidth;
            const incomeHeight = (trend.income / maxValue) * chartHeight;
            const expensesHeight = (trend.expenses / maxValue) * chartHeight;

            return (
              <g
                key={trend.month}
                className={styles.barGroup}
                onMouseEnter={(e) => handleMouseEnter(trend, index, e)}
                onMouseLeave={handleMouseLeave}
              >
                {/* Income bar */}
                <rect
                  className={`${styles.bar} ${styles.income}`}
                  x={groupX + barGap}
                  y={padding.top + chartHeight - incomeHeight}
                  width={barWidth}
                  height={incomeHeight}
                  rx={2}
                />
                {/* Expenses bar */}
                <rect
                  className={`${styles.bar} ${styles.expenses}`}
                  x={groupX + barWidth + barGap * 2}
                  y={padding.top + chartHeight - expensesHeight}
                  width={barWidth}
                  height={expensesHeight}
                  rx={2}
                />
                {/* Month label */}
                <text
                  className={styles.axisLabel}
                  x={groupX + barGroupWidth / 2}
                  y={height - 5}
                  textAnchor="middle"
                >
                  {trend.monthLabel}
                </text>
              </g>
            );
          })}
        </svg>

        {/* Tooltip */}
        {tooltip && (
          <div
            className={styles.tooltip}
            style={{
              left: tooltip.x,
              top: tooltip.y,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div className={styles.month}>{tooltip.month}</div>
            <div className={styles.values}>
              <span className={styles.income}>
                Income: {formatCurrency(tooltip.income)}
              </span>
              <span className={styles.expenses}>
                Expenses: {formatCurrency(tooltip.expenses)}
              </span>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
