import { useState, type ReactNode } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';
import { ChevronRight } from 'lucide-react-native';
import { colors, radii, spacing } from '../theme';
import { formatCompactCurrency, formatCurrency, formatMonthName } from '../utils/date';
import { AppText } from './AppText';
import { Hatch } from './Hatch';

/** A title above a group of cards, with an optional "View all". */
export function SectionHead({ title, onViewAll, right }: { title: string; onViewAll?: () => void; right?: ReactNode }) {
  return (
    <View style={styles.sectionHead}>
      <AppText variant="heading" style={styles.grow}>
        {title}
      </AppText>
      {right}
      {onViewAll ? (
        <Pressable style={styles.inline} onPress={onViewAll} hitSlop={10} accessibilityRole="button">
          <AppText variant="caption" muted>
            View all
          </AppText>
          <ChevronRight size={14} color={colors.muted} />
        </Pressable>
      ) : null}
    </View>
  );
}

/** Round outlined button, the chevron/arrow on the right of a card. */
export function CircleButton({ children, onPress, label, size = 34 }: { children: ReactNode; onPress?: () => void; label: string; size?: number }) {
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      hitSlop={6}
      accessibilityRole="button"
      accessibilityLabel={label}
      style={({ pressed }) => [styles.circle, { width: size, height: size, borderRadius: size / 2 }, pressed && styles.pressed]}
    >
      {children}
    </Pressable>
  );
}

/** Rounded-square outlined tile holding a letter, for a category or payee. */
export function LetterTile({ name, color, size = 40 }: { name: string; color: string; size?: number }) {
  return (
    <View style={[styles.letterTile, { width: size, height: size, borderRadius: size * 0.3 }]}>
      <AppText weight="bold" style={{ color, fontSize: size * 0.4, lineHeight: size * 0.5 }}>
        {(name.trim().charAt(0) || '?').toUpperCase()}
      </AppText>
    </View>
  );
}

export type PillBarPoint = { month: string; value: number };

/**
 * Hatched pill bars, one per month; the last month is solid accent. Tapping a
 * bar shows its amount in the top-right corner.
 */
export function PillBars({ title, points, height = 120 }: { title: string; points: PillBarPoint[]; height?: number }) {
  const [selected, setSelected] = useState<string | null>(null);
  if (points.length < 2) return null;
  const max = Math.max(...points.map((p) => p.value), 1);
  const earlier = points.slice(0, -1).filter((p) => p.value > 0);
  const average = earlier.length ? earlier.reduce((sum, p) => sum + p.value, 0) / earlier.length : 0;
  const current = points[points.length - 1].month;
  const pick = points.find((p) => p.month === (selected ?? current));

  return (
    <View style={styles.inset}>
      <View style={styles.inline}>
        <View style={styles.grow}>
          <AppText weight="semibold">{title}</AppText>
          {average > 0 ? (
            <AppText variant="caption" muted>
              Average per month {formatCurrency(average)}
            </AppText>
          ) : null}
        </View>
        {pick ? (
          <View style={styles.end}>
            <AppText variant="caption" tone="faint">
              {formatMonthName(pick.month, 'short')}
            </AppText>
            <AppText weight="bold" tabular>
              {formatCompactCurrency(pick.value)}
            </AppText>
          </View>
        ) : null}
      </View>
      <View style={styles.bars}>
        {points.map((p) => {
          const isCurrent = p.month === current;
          const isPicked = p.month === pick?.month;
          const barHeight = Math.max(14, (p.value / max) * height);
          return (
            <Pressable
              key={p.month}
              style={styles.barColumn}
              onPress={() => setSelected(p.month)}
              accessibilityRole="button"
              accessibilityLabel={`${formatMonthName(p.month)}: ${formatCurrency(p.value)}`}
            >
              <View style={[styles.barSlot, { height }]}>
                <View style={[styles.pillBar, { height: barHeight }, isCurrent && styles.pillBarCurrent, isPicked && !isCurrent && styles.pillBarPicked]}>
                  {isCurrent ? null : <Hatch color={colors.barHatch} gap={5} width={1.2} />}
                </View>
              </View>
              <AppText variant="caption" tone={isPicked ? 'default' : 'faint'} weight={isPicked ? 'semibold' : undefined}>
                {formatMonthName(p.month, 'short')}
              </AppText>
            </Pressable>
          );
        })}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  grow: {
    flex: 1,
    minWidth: 0
  },
  inline: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4
  },
  end: {
    alignItems: 'flex-end'
  },
  pressed: {
    opacity: 0.7
  },
  sectionHead: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    paddingHorizontal: spacing.xs,
    marginTop: spacing.xs
  },
  circle: {
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 1,
    borderColor: colors.borderLight,
    backgroundColor: colors.surface
  },
  letterTile: {
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface
  },
  inset: {
    gap: spacing.sm,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    padding: 10
  },
  bars: {
    flexDirection: 'row',
    justifyContent: 'space-between'
  },
  barColumn: {
    flex: 1,
    alignItems: 'center',
    gap: spacing.sm
  },
  barSlot: {
    justifyContent: 'flex-end'
  },
  pillBar: {
    width: 30,
    borderRadius: 15,
    backgroundColor: colors.barStrong,
    overflow: 'hidden'
  },
  pillBarCurrent: {
    backgroundColor: colors.primary
  },
  pillBarPicked: {
    opacity: 0.75
  }
});
