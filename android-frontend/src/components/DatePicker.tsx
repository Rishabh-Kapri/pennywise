import { useMemo, useState } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';
import { ChevronLeft, ChevronRight } from 'lucide-react-native';
import { AppText } from './AppText';
import { SheetOverlay } from './Sheet';
import { colors, radii, spacing } from '../theme';
import { addDays, todayKey } from '../utils/date';

const WEEKDAYS = ['S', 'M', 'T', 'W', 'T', 'F', 'S'];

function monthTitle(year: number, month: number) {
  return new Intl.DateTimeFormat('en-IN', { month: 'long', year: 'numeric' }).format(new Date(year, month, 1));
}

function dayKey(year: number, month: number, day: number) {
  return `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
}

/** Month grid for a YYYY-MM-DD value, opening on the selected month. */
export function Calendar({ value, onChange }: { value: string; onChange: (date: string) => void }) {
  const [cursor, setCursor] = useState(() => {
    const [year, month] = (value || todayKey()).split('-').map(Number);
    return { year, month: month - 1 };
  });
  const today = todayKey();

  const cells = useMemo(() => {
    const first = new Date(cursor.year, cursor.month, 1).getDay();
    const days = new Date(cursor.year, cursor.month + 1, 0).getDate();
    const list: (string | null)[] = Array.from({ length: first }, () => null);
    for (let day = 1; day <= days; day++) list.push(dayKey(cursor.year, cursor.month, day));
    while (list.length % 7) list.push(null);
    return list;
  }, [cursor]);

  const shift = (delta: number) =>
    setCursor(({ year, month }) => {
      const date = new Date(year, month + delta, 1);
      return { year: date.getFullYear(), month: date.getMonth() };
    });

  const weeks: (string | null)[][] = [];
  for (let i = 0; i < cells.length; i += 7) weeks.push(cells.slice(i, i + 7));

  return (
    <View style={styles.calendar}>
      <View style={styles.monthRow}>
        <Pressable accessibilityLabel="Previous month" hitSlop={8} style={styles.navButton} onPress={() => shift(-1)}>
          <ChevronLeft size={18} color={colors.text} />
        </Pressable>
        <AppText weight="semibold">{monthTitle(cursor.year, cursor.month)}</AppText>
        <Pressable accessibilityLabel="Next month" hitSlop={8} style={styles.navButton} onPress={() => shift(1)}>
          <ChevronRight size={18} color={colors.text} />
        </Pressable>
      </View>
      <View style={styles.week}>
        {WEEKDAYS.map((day, index) => (
          <View key={index} style={styles.cell}>
            <AppText variant="caption" tone="faint" weight="semibold">
              {day}
            </AppText>
          </View>
        ))}
      </View>
      {weeks.map((week, row) => (
        <View key={row} style={styles.week}>
          {week.map((date, index) => {
            if (!date) return <View key={index} style={styles.cell} />;
            const selected = date === value;
            const isToday = date === today;
            return (
              <Pressable
                key={date}
                accessibilityRole="button"
                accessibilityLabel={date}
                accessibilityState={{ selected }}
                style={styles.cell}
                onPress={() => onChange(date)}
              >
                <View style={[styles.day, isToday && styles.today, selected && styles.selected]}>
                  <AppText
                    tabular
                    weight={selected || isToday ? 'semibold' : 'regular'}
                    style={selected ? styles.selectedText : isToday ? styles.todayText : undefined}
                  >
                    {Number(date.slice(8))}
                  </AppText>
                </View>
              </Pressable>
            );
          })}
        </View>
      ))}
    </View>
  );
}

/** Calendar in a sheet, with one-tap Today / Yesterday. Picking a day closes it. */
export function DatePickerOverlay({
  value,
  onSelect,
  onClose,
  title = 'Date'
}: {
  value: string;
  onSelect: (date: string) => void;
  onClose: () => void;
  title?: string;
}) {
  const today = todayKey();
  const shortcuts = [
    { label: 'Today', date: today },
    { label: 'Yesterday', date: addDays(today, -1) },
    { label: '2 days ago', date: addDays(today, -2) }
  ];

  return (
    <SheetOverlay title={title} onClose={onClose}>
      <View style={styles.body}>
        <View style={styles.shortcuts}>
          {shortcuts.map((shortcut) => {
            const active = shortcut.date === value;
            return (
              <Pressable
                key={shortcut.label}
                style={({ pressed }) => [styles.chip, active && styles.chipActive, pressed && styles.pressed]}
                onPress={() => onSelect(shortcut.date)}
              >
                <AppText variant="caption" weight="semibold" style={active ? styles.chipActiveText : styles.chipText}>
                  {shortcut.label}
                </AppText>
              </Pressable>
            );
          })}
        </View>
        <Calendar value={value} onChange={onSelect} />
      </View>
    </SheetOverlay>
  );
}

const styles = StyleSheet.create({
  body: {
    gap: spacing.lg,
    paddingBottom: spacing.sm
  },
  shortcuts: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  chip: {
    minHeight: 34,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong
  },
  chipActive: {
    backgroundColor: colors.primaryMuted
  },
  chipText: {
    color: colors.muted
  },
  chipActiveText: {
    color: colors.primary
  },
  pressed: {
    opacity: 0.7
  },
  calendar: {
    gap: spacing.xs
  },
  monthRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: spacing.xs
  },
  navButton: {
    width: 36,
    height: 36,
    borderRadius: 18,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  week: {
    flexDirection: 'row'
  },
  cell: {
    flex: 1,
    height: 44,
    alignItems: 'center',
    justifyContent: 'center'
  },
  day: {
    width: 40,
    height: 40,
    borderRadius: 20,
    alignItems: 'center',
    justifyContent: 'center'
  },
  today: {
    borderWidth: 1,
    borderColor: colors.primary
  },
  todayText: {
    color: colors.primary
  },
  selected: {
    backgroundColor: colors.primary,
    borderWidth: 0
  },
  selectedText: {
    color: colors.onPrimary
  }
});
