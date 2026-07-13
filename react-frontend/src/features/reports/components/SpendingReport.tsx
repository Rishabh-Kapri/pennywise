import { useMemo, useState } from 'react';
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts';
import { ArrowLeft } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { formatCurrency } from '@/features/dashboard/utils';
import { getSelectedMonthInHumanFormat } from '@/utils/date.utils';
import { selectSpendingLoading, selectSpendingReport } from '../store/reportSlice';
import styles from './SpendingReport.module.css';

// Categorical palette in fixed assignment order, CVD-validated against the
// card surface (#323232). Hex because SVG fills can't resolve CSS vars.
const SERIES_COLORS = [
  '#3987e5',
  '#c08120',
  '#1baf7a',
  '#c356a7',
  '#8f7fe8',
  '#d55b33',
  '#279cb2',
  '#8f9a35',
];
const OTHER_COLOR = '#8a8a8a';
const SURFACE_COLOR = '#323232';
const MAX_SLICES = SERIES_COLORS.length;

interface Slice {
  id: string;
  name: string;
  total: number;
  color: string;
  drillable: boolean;
}

// Fold anything beyond the fixed palette into a gray "Other" slice —
// categorical hues are never cycled.
function toSlices(items: { id: string; name: string; total: number; drillable: boolean }[]): Slice[] {
  const head = items.slice(0, items.length > MAX_SLICES ? MAX_SLICES - 1 : MAX_SLICES);
  const rest = items.slice(head.length);
  const slices: Slice[] = head.map((item, i) => ({ ...item, color: SERIES_COLORS[i] }));
  if (rest.length > 0) {
    slices.push({
      id: '__other__',
      name: `Other (${rest.length})`,
      total: rest.reduce((sum, item) => sum + item.total, 0),
      color: OTHER_COLOR,
      drillable: false,
    });
  }
  return slices;
}

interface SliceTooltipProps {
  active?: boolean;
  payload?: { payload?: Slice }[];
}

const SliceTooltip = ({ active, payload }: SliceTooltipProps) => {
  const slice = payload?.[0]?.payload;
  if (!active || !slice) return null;
  return (
    <div className={styles.tooltip}>
      <span className={styles.tooltipSwatch} style={{ background: slice.color }} />
      <span className={styles.tooltipName}>{slice.name}</span>
      <span className={styles.tooltipValue}>{formatCurrency(slice.total)}</span>
    </div>
  );
};

export default function SpendingReport() {
  const report = useAppSelector(selectSpendingReport);
  const loading = useAppSelector(selectSpendingLoading);
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);

  const selectedGroup = useMemo(
    () => report?.groups.find((g) => g.categoryGroupId === selectedGroupId) ?? null,
    [report, selectedGroupId],
  );

  const slices = useMemo(() => {
    if (!report) return [];
    if (selectedGroup) {
      return toSlices(
        selectedGroup.categories.map((c) => ({
          id: c.categoryId,
          name: c.name,
          total: c.total,
          drillable: false,
        })),
      );
    }
    return toSlices(
      report.groups.map((g) => ({
        id: g.categoryGroupId,
        name: g.name,
        total: g.total,
        drillable: true,
      })),
    );
  }, [report, selectedGroup]);

  if (loading === LoadingState.PENDING || loading === LoadingState.IDLE) {
    return <div className={styles.emptyState}>Loading spending report…</div>;
  }
  if (!report || report.groups.length === 0) {
    return <div className={styles.emptyState}>No spending in the selected period</div>;
  }

  const total = selectedGroup ? selectedGroup.total : report.total;
  const rangeLabel = `${getSelectedMonthInHumanFormat(report.startMonth)} – ${getSelectedMonthInHumanFormat(report.endMonth)}`;

  const handleSliceClick = (slice: Slice) => {
    if (!selectedGroup && slice.drillable) {
      setSelectedGroupId(slice.id);
    }
  };

  return (
    <div className={styles.card}>
      <div className={styles.breadcrumb}>
        {selectedGroup ? (
          <>
            <button className={styles.backBtn} onClick={() => setSelectedGroupId(null)}>
              <ArrowLeft size={16} />
              All groups
            </button>
            <span className={styles.breadcrumbSeparator}>/</span>
            <span className={styles.breadcrumbCurrent}>{selectedGroup.name}</span>
          </>
        ) : (
          <span className={styles.breadcrumbCurrent}>Spending by category group</span>
        )}
        <span className={styles.rangeLabel}>{rangeLabel}</span>
      </div>

      <div className={styles.chartLayout}>
        <div className={styles.donutWrap}>
          <ResponsiveContainer width="100%" height={280} initialDimension={{ width: 280, height: 280 }}>
            <PieChart>
              <Tooltip content={<SliceTooltip />} />
              <Pie
                data={slices}
                dataKey="total"
                nameKey="name"
                innerRadius="62%"
                outerRadius="90%"
                stroke={SURFACE_COLOR}
                strokeWidth={2}
                isAnimationActive={false}
                onClick={(_, index) => handleSliceClick(slices[index])}>
                {slices.map((slice) => (
                  <Cell
                    key={slice.id}
                    fill={slice.color}
                    cursor={slice.drillable ? 'pointer' : 'default'}
                  />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
          <div className={styles.donutCenter}>
            <span className={styles.donutTotal}>{formatCurrency(total)}</span>
            <span className={styles.donutCaption}>{selectedGroup ? selectedGroup.name : 'Total spending'}</span>
          </div>
        </div>

        <ul className={styles.legendList}>
          {slices.map((slice) => (
            <li key={slice.id}>
              <button
                className={slice.drillable ? styles.legendRowDrillable : styles.legendRow}
                onClick={() => handleSliceClick(slice)}
                disabled={!slice.drillable}>
                <span className={styles.legendDot} style={{ background: slice.color }} />
                <span className={styles.legendName}>{slice.name}</span>
                <span className={styles.legendPercent}>
                  {total > 0 ? `${((slice.total / total) * 100).toFixed(1)}%` : '—'}
                </span>
                <span className={styles.legendValue}>{formatCurrency(slice.total)}</span>
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
