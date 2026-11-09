import { useRef, useState, type ReactNode } from 'react';
import { Pressable, ScrollView, StyleSheet, View, type LayoutChangeEvent } from 'react-native';
import Svg, { Circle, Line, Path, Polyline } from 'react-native-svg';
import { AppText } from './AppText';
import { chartColors, colors, radii, spacing } from '../theme';
import { formatCompactCurrency } from '../utils/date';

// Charts for the reports, dashboard and loans. Bars are plain Views; lines and
// rings use react-native-svg, which lucide-react-native already ships in the
// native build. On a phone there is no hover, so a tap selects a column and
// the caller shows that column's numbers under the chart.

export type ChartSeries = { key: string; label: string; color: string };
export type ChartColumn = { key: string; label: string; values: Record<string, number> };

type BarChartProps = {
  columns: ChartColumn[];
  series: ChartSeries[];
  /** stacked: one bar per column; grouped: side by side per series */
  mode?: 'stacked' | 'grouped';
  height?: number;
  selected?: number | null;
  onSelect?: (index: number) => void;
};

const AXIS_WIDTH = 46;
const MIN_COLUMN = 30;

function niceMax(value: number) {
  if (value <= 0) return 1;
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const step = [1, 1.25, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10].find((m) => m * magnitude >= value) ?? 10;
  return step * magnitude;
}

/**
 * Vertical bars per column. Negative values hang below the zero line (e.g.
 * net withdrawn). Columns scroll sideways once they'd be narrower than a tap.
 */
export function BarChart({ columns, series, mode = 'stacked', height = 200, selected, onSelect }: BarChartProps) {
  const [width, setWidth] = useState(0);
  const scrollRef = useRef<ScrollView>(null);
  const scrolled = useRef(false);

  let top = 0;
  let bottom = 0;
  for (const column of columns) {
    if (mode === 'stacked') {
      let pos = 0;
      let neg = 0;
      for (const s of series) {
        const v = column.values[s.key] ?? 0;
        if (v >= 0) pos += v;
        else neg += v;
      }
      top = Math.max(top, pos);
      bottom = Math.min(bottom, neg);
    } else {
      for (const s of series) {
        const v = column.values[s.key] ?? 0;
        top = Math.max(top, v);
        bottom = Math.min(bottom, v);
      }
    }
  }
  const maxTop = niceMax(top);
  const maxBottom = bottom < 0 ? niceMax(-bottom) : 0;
  const span = maxTop + maxBottom || 1;
  const zeroY = (maxTop / span) * height;
  const scale = (value: number) => (Math.abs(value) / span) * height;

  const plotWidth = Math.max(width - AXIS_WIDTH, 0);
  const columnWidth = columns.length ? Math.max(plotWidth / columns.length, MIN_COLUMN) : 0;
  const contentWidth = columnWidth * columns.length;
  const barWidth = Math.min(columnWidth * (mode === 'grouped' ? 0.36 : 0.56), mode === 'grouped' ? 12 : 22);

  const ticks = [maxTop, maxTop / 2, 0, ...(maxBottom ? [-maxBottom] : [])];

  const renderColumn = (column: ChartColumn, index: number) => {
    const isSelected = selected === index;
    const dim = selected != null && !isSelected;
    let bars: ReactNode;
    if (mode === 'stacked') {
      let up = 0;
      let down = 0;
      bars = series.map((s) => {
        const v = column.values[s.key] ?? 0;
        if (!v) return null;
        const h = scale(v);
        const y = v >= 0 ? zeroY - up - h : zeroY + down;
        if (v >= 0) up += h;
        else down += h;
        return (
          <View
            key={s.key}
            style={[styles.bar, { top: y, height: Math.max(h, 1), width: barWidth, backgroundColor: s.color }]}
          />
        );
      });
    } else {
      bars = (
        <View style={[styles.group, { width: barWidth * series.length + 2 * (series.length - 1) }]}>
          {series.map((s, i) => {
            const v = column.values[s.key] ?? 0;
            const h = scale(v);
            return (
              <View
                key={s.key}
                style={[
                  styles.bar,
                  {
                    left: i * (barWidth + 2),
                    top: v >= 0 ? zeroY - h : zeroY,
                    height: v ? Math.max(h, 1) : 0,
                    width: barWidth,
                    backgroundColor: s.color
                  }
                ]}
              />
            );
          })}
        </View>
      );
    }
    return (
      <Pressable
        key={column.key}
        accessibilityRole="button"
        accessibilityLabel={column.label}
        onPress={() => onSelect?.(index)}
        style={[styles.column, { width: columnWidth }]}
      >
        <View style={[styles.plot, { height }, isSelected && styles.selectedColumn, dim && styles.dim]}>{bars}</View>
        <AppText variant="caption" tone={isSelected ? 'default' : 'faint'} numberOfLines={1} style={styles.xLabel}>
          {column.label}
        </AppText>
      </Pressable>
    );
  };

  return (
    <View onLayout={(event: LayoutChangeEvent) => setWidth(event.nativeEvent.layout.width)} style={styles.chart}>
      <View style={[styles.axis, { height }]}>
        {ticks.map((tick) => (
          <AppText
            key={tick}
            variant="caption"
            tone="faint"
            tabular
            style={[styles.tick, { top: zeroY - (tick / span) * height - 8 }]}
          >
            {formatCompactCurrency(tick)}
          </AppText>
        ))}
      </View>
      <View style={styles.plotArea}>
        {ticks.map((tick) => (
          <View
            key={tick}
            style={[styles.gridLine, tick === 0 && styles.zeroLine, { top: zeroY - (tick / span) * height }]}
          />
        ))}
        {width ? (
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={{ width: Math.max(contentWidth, plotWidth) }}
            ref={scrollRef}
            // start at the latest month; only on first layout so a tap doesn't jump
            onContentSizeChange={() => {
              if (scrolled.current || contentWidth <= plotWidth) return;
              scrolled.current = true;
              scrollRef.current?.scrollToEnd({ animated: false });
            }}
          >
            {columns.map(renderColumn)}
          </ScrollView>
        ) : null}
      </View>
    </View>
  );
}

export type LinePoint = { key: string; label: string; values: Record<string, number> };

/**
 * Lines over months; the first series gets a soft area fill. Tapping a point
 * selects it.
 */
export function LineChart({
  points,
  series,
  height = 200,
  selected,
  onSelect
}: {
  points: LinePoint[];
  series: ChartSeries[];
  height?: number;
  selected?: number | null;
  onSelect?: (index: number) => void;
}) {
  const [width, setWidth] = useState(0);
  const values = points.flatMap((point) => series.map((s) => point.values[s.key] ?? 0));
  const rawMax = Math.max(...values, 0);
  const rawMin = Math.min(...values, 0);
  const max = niceMax(rawMax);
  const min = rawMin < 0 ? -niceMax(-rawMin) : 0;
  const span = max - min || 1;
  const plotWidth = Math.max(width - AXIS_WIDTH, 0);
  const stepX = points.length > 1 ? plotWidth / (points.length - 1) : 0;
  const x = (i: number) => (points.length > 1 ? i * stepX : plotWidth / 2);
  const y = (v: number) => height - ((v - min) / span) * height;
  const ticks = [max, (max + min) / 2, min];
  const labelEvery = Math.max(1, Math.ceil(points.length / 6));

  return (
    <View onLayout={(event) => setWidth(event.nativeEvent.layout.width)}>
      <View style={styles.chart}>
        <View style={[styles.axis, { height }]}>
          {ticks.map((tick) => (
            <AppText key={tick} variant="caption" tone="faint" tabular style={[styles.tick, { top: y(tick) - 8 }]}>
              {formatCompactCurrency(tick)}
            </AppText>
          ))}
        </View>
        <View style={[styles.plotArea, { height }]}>
          {plotWidth > 0 ? (
            <Svg width={plotWidth} height={height}>
              {ticks.map((tick) => (
                <Line key={tick} x1={0} x2={plotWidth} y1={y(tick)} y2={y(tick)} stroke={chartColors.grid} strokeWidth={1} />
              ))}
              {series.map((s, si) => {
                const coords = points.map((point, i) => `${x(i)},${y(point.values[s.key] ?? 0)}`);
                return si === 0 && points.length > 1 ? (
                  <Path
                    key={`${s.key}-area`}
                    d={`M${x(0)},${y(Math.max(min, 0))} L${coords.join(' L')} L${x(points.length - 1)},${y(Math.max(min, 0))} Z`}
                    fill={s.color}
                    fillOpacity={0.12}
                  />
                ) : null;
              })}
              {series.map((s) => (
                <Polyline
                  key={s.key}
                  points={points.map((point, i) => `${x(i)},${y(point.values[s.key] ?? 0)}`).join(' ')}
                  fill="none"
                  stroke={s.color}
                  strokeWidth={2}
                  strokeLinejoin="round"
                />
              ))}
              {selected != null && points[selected] ? (
                <>
                  <Line x1={x(selected)} x2={x(selected)} y1={0} y2={height} stroke={colors.borderLight} strokeWidth={1} />
                  {series.map((s) => (
                    <Circle
                      key={s.key}
                      cx={x(selected)}
                      cy={y(points[selected].values[s.key] ?? 0)}
                      r={4}
                      fill={s.color}
                      stroke={colors.surface}
                      strokeWidth={2}
                    />
                  ))}
                </>
              ) : null}
            </Svg>
          ) : null}
          {/* tap targets, one per point */}
          <View style={StyleSheet.absoluteFill}>
            <View style={styles.tapRow}>
              {points.map((point, i) => (
                <Pressable
                  key={point.key}
                  accessibilityLabel={point.label}
                  style={styles.tapTarget}
                  onPress={() => onSelect?.(i)}
                />
              ))}
            </View>
          </View>
        </View>
      </View>
      <View style={[styles.lineLabels, { marginLeft: AXIS_WIDTH }]}>
        {points.map((point, i) =>
          i % labelEvery === 0 || i === points.length - 1 ? (
            <AppText
              key={point.key}
              variant="caption"
              tone="faint"
              style={[styles.lineLabel, { left: Math.min(Math.max(x(i) - 24, 0), plotWidth - 48) }]}
            >
              {point.label}
            </AppText>
          ) : null
        )}
      </View>
    </View>
  );
}

/** Progress ring with the percentage in the middle. */
export function Ring({
  percent,
  size = 120,
  stroke = 10,
  color = colors.primary,
  children
}: {
  percent: number;
  size?: number;
  stroke?: number;
  color?: string;
  children?: ReactNode;
}) {
  const r = (size - stroke) / 2;
  const circumference = 2 * Math.PI * r;
  const clamped = Math.max(0, Math.min(percent, 100));
  return (
    <View style={{ width: size, height: size, alignItems: 'center', justifyContent: 'center' }}>
      <Svg width={size} height={size} style={StyleSheet.absoluteFill}>
        <Circle cx={size / 2} cy={size / 2} r={r} stroke={colors.surfaceTertiary} strokeWidth={stroke} fill="none" />
        <Circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke={color}
          strokeWidth={stroke}
          fill="none"
          strokeLinecap="round"
          strokeDasharray={`${circumference} ${circumference}`}
          strokeDashoffset={circumference * (1 - clamped / 100)}
          transform={`rotate(-90 ${size / 2} ${size / 2})`}
        />
      </Svg>
      {children}
    </View>
  );
}

export function Legend({ items }: { items: { label: string; color: string; value?: string }[] }) {
  return (
    <View style={styles.legend}>
      {items.map((item) => (
        <View key={item.label} style={styles.legendItem}>
          <View style={[styles.swatch, { backgroundColor: item.color }]} />
          <AppText variant="caption" muted numberOfLines={1}>
            {item.label}
            {item.value ? (
              <AppText variant="caption" weight="semibold">
                {` ${item.value}`}
              </AppText>
            ) : null}
          </AppText>
        </View>
      ))}
    </View>
  );
}

/** One stacked horizontal bar, e.g. where the money went. */
export function StackBar({
  segments,
  scale,
  height = 14
}: {
  segments: { key: string; value: number; color: string }[];
  /** the value that fills the whole track */
  scale: number;
  height?: number;
}) {
  return (
    <View style={[styles.stackTrack, { height, borderRadius: height / 2 }]}>
      {segments
        .filter((segment) => segment.value > 0)
        .map((segment) => (
          <View
            key={segment.key}
            style={{ width: `${Math.min((segment.value / (scale || 1)) * 100, 100)}%`, backgroundColor: segment.color }}
          />
        ))}
    </View>
  );
}

/** Thin bar showing a value against the largest row. */
export function RowBar({ value, max, color }: { value: number; max: number; color: string }) {
  const width = max > 0 ? Math.max(0, Math.min(value / max, 1)) * 100 : 0;
  return (
    <View style={styles.rowTrack}>
      <View style={[styles.rowFill, { width: `${width}%`, backgroundColor: color }]} />
    </View>
  );
}

export function Segmented<T extends string>({
  options,
  value,
  onChange
}: {
  options: readonly { value: T; label: string }[];
  value: T;
  onChange: (value: T) => void;
}) {
  return (
    <View style={styles.segmented}>
      {options.map((option) => {
        const active = option.value === value;
        return (
          <Pressable
            key={option.value}
            accessibilityRole="button"
            accessibilityState={{ selected: active }}
            style={[styles.segment, active && styles.segmentActive]}
            onPress={() => onChange(option.value)}
          >
            <AppText variant="caption" weight="semibold" style={active ? styles.segmentActiveText : styles.segmentText}>
              {option.label}
            </AppText>
          </Pressable>
        );
      })}
    </View>
  );
}

/** Label, big figure, one line of context. */
export function Stat({
  label,
  value,
  detail,
  color,
  tone
}: {
  label: string;
  value: string;
  detail?: ReactNode;
  color?: string;
  tone?: 'success' | 'danger';
}) {
  return (
    <View style={styles.stat}>
      <View style={styles.statLabel}>
        {color ? <View style={[styles.swatch, { backgroundColor: color }]} /> : null}
        <AppText variant="caption" muted numberOfLines={1}>
          {label}
        </AppText>
      </View>
      <AppText variant="heading" tabular tone={tone} numberOfLines={1} adjustsFontSizeToFit>
        {value}
      </AppText>
      {detail ? (
        <AppText variant="caption" tone="faint">
          {detail}
        </AppText>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  chart: {
    flexDirection: 'row'
  },
  axis: {
    width: AXIS_WIDTH,
    position: 'relative'
  },
  tick: {
    position: 'absolute',
    left: 0,
    fontSize: 11
  },
  plotArea: {
    flex: 1,
    position: 'relative'
  },
  gridLine: {
    position: 'absolute',
    left: 0,
    right: 0,
    height: StyleSheet.hairlineWidth,
    backgroundColor: chartColors.grid
  },
  zeroLine: {
    backgroundColor: colors.borderLight
  },
  column: {
    alignItems: 'center'
  },
  plot: {
    alignSelf: 'stretch',
    alignItems: 'center',
    borderRadius: radii.sm
  },
  selectedColumn: {
    backgroundColor: 'rgba(255,255,255,0.05)'
  },
  dim: {
    opacity: 0.55
  },
  group: {
    height: '100%'
  },
  bar: {
    position: 'absolute',
    borderRadius: 3
  },
  xLabel: {
    fontSize: 11,
    marginTop: 4
  },
  tapRow: {
    flex: 1,
    flexDirection: 'row'
  },
  tapTarget: {
    flex: 1
  },
  lineLabels: {
    height: 18,
    marginTop: 4,
    position: 'relative'
  },
  lineLabel: {
    position: 'absolute',
    width: 48,
    textAlign: 'center',
    fontSize: 11
  },
  legend: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.md
  },
  legendItem: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    maxWidth: '100%'
  },
  swatch: {
    width: 9,
    height: 9,
    borderRadius: 3
  },
  stackTrack: {
    flexDirection: 'row',
    overflow: 'hidden',
    backgroundColor: colors.surfaceStrong,
    gap: 1
  },
  rowTrack: {
    height: 5,
    borderRadius: 3,
    backgroundColor: colors.surfaceStrong,
    overflow: 'hidden'
  },
  rowFill: {
    height: '100%',
    borderRadius: 3
  },
  segmented: {
    flexDirection: 'row',
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.full,
    padding: 3
  },
  segment: {
    minHeight: 30,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: radii.full
  },
  segmentActive: {
    backgroundColor: colors.surfaceTertiary
  },
  segmentText: {
    color: colors.muted
  },
  segmentActiveText: {
    color: colors.text
  },
  stat: {
    flex: 1,
    minWidth: 140,
    gap: 2,
    backgroundColor: colors.surface,
    borderRadius: radii.lg,
    padding: spacing.lg
  },
  statLabel: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6
  }
});
