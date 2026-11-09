import { useMemo, useState } from 'react';
import { FlatList, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { Check, ChevronDown, Plus, Search, X } from 'lucide-react-native';
import { AppText } from './AppText';
import { Sheet, SheetOverlay } from './Sheet';
import { colors, radii, spacing } from '../theme';

export type PickerOption = {
  id: string;
  label: string;
  sublabel?: string;
  color?: string;
  /** options sharing a group are listed under one header, in the order given */
  group?: string;
};

/** Tappable form row that shows the current selection and opens a picker. */
export function PickerField({
  label,
  value,
  placeholder = 'Select',
  onPress
}: {
  label: string;
  value?: string | null;
  placeholder?: string;
  onPress: () => void;
}) {
  return (
    <View style={styles.field}>
      <AppText variant="label" tone="faint">
        {label}
      </AppText>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={`${label}: ${value ?? placeholder}`}
        style={({ pressed }) => [styles.fieldButton, pressed && styles.pressed]}
        onPress={onPress}
      >
        <AppText numberOfLines={1} style={[styles.fieldValue, !value && styles.fieldPlaceholder]}>
          {value || placeholder}
        </AppText>
        <ChevronDown size={16} color={colors.faint} />
      </Pressable>
    </View>
  );
}

type PickerListProps = {
  title: string;
  options: PickerOption[];
  selectedId?: string | null;
  onSelect: (id: string | null) => void;
  onClose: () => void;
  searchPlaceholder?: string;
  /** adds a "none" row on top that selects null */
  clearLabel?: string;
  /** offers to create the typed name when nothing matches it exactly */
  onCreate?: (name: string) => void;
  autoFocusSearch?: boolean;
};

type Row =
  | { kind: 'clear'; key: string }
  | { kind: 'create'; key: string; name: string }
  | { kind: 'header'; key: string; label: string }
  | { kind: 'option'; key: string; option: PickerOption };

// Fixed heights let the list jump straight to the selected row on open.
const ROW_HEIGHT = 52;
const HEADER_HEIGHT = 34;
const rowHeight = (row: Row) => (row.kind === 'header' ? HEADER_HEIGHT : ROW_HEIGHT);

function buildRows(options: PickerOption[], query: string, clearLabel?: string, canCreate?: boolean): Row[] {
  const q = query.trim().toLowerCase();
  const matches = q
    ? options.filter(
        (option) => option.label.toLowerCase().includes(q) || (option.sublabel ?? '').toLowerCase().includes(q)
      )
    : options;

  const rows: Row[] = [];
  if (canCreate && q && !options.some((option) => option.label.trim().toLowerCase() === q)) {
    rows.push({ kind: 'create', key: '__create__', name: query.trim() });
  }
  if (clearLabel && !q) rows.push({ kind: 'clear', key: '__clear__' });

  let group: string | undefined;
  for (const option of matches) {
    if (option.group !== undefined && option.group !== group) {
      group = option.group;
      rows.push({ kind: 'header', key: `__group__${group}`, label: group });
    }
    rows.push({ kind: 'option', key: option.id, option });
  }
  return rows;
}

function PickerBody({
  options,
  selectedId,
  onSelect,
  searchPlaceholder = 'Search',
  clearLabel,
  onCreate,
  autoFocusSearch
}: Omit<PickerListProps, 'title' | 'onClose'>) {
  const [query, setQuery] = useState('');
  const rows = useMemo(() => buildRows(options, query, clearLabel, Boolean(onCreate)), [options, query, clearLabel, onCreate]);

  const offsets = useMemo(() => {
    const result: number[] = [];
    let offset = 0;
    for (const row of rows) {
      result.push(offset);
      offset += rowHeight(row);
    }
    return result;
  }, [rows]);

  // Open on the current choice with a couple of rows of context above it.
  // Computed once: later filtering shouldn't yank the list around.
  const [initialIndex] = useState(() => {
    const index = rows.findIndex((row) => row.kind === 'option' && row.option.id === selectedId);
    return index > 2 ? index - 2 : undefined;
  });

  return (
    <View style={styles.body}>
      <View style={styles.searchBox}>
        <Search size={16} color={colors.faint} />
        <TextInput
          value={query}
          onChangeText={setQuery}
          placeholder={searchPlaceholder}
          placeholderTextColor={colors.faint}
          style={styles.searchInput}
          autoCorrect={false}
          autoFocus={autoFocusSearch}
          returnKeyType="done"
        />
        {query ? (
          <Pressable accessibilityLabel="Clear search" hitSlop={8} onPress={() => setQuery('')}>
            <X size={16} color={colors.faint} />
          </Pressable>
        ) : null}
      </View>

      <FlatList
        data={rows}
        style={styles.list}
        keyExtractor={(row) => row.key}
        keyboardShouldPersistTaps="handled"
        keyboardDismissMode="on-drag"
        showsVerticalScrollIndicator={false}
        initialScrollIndex={initialIndex}
        getItemLayout={(_, index) => ({
          length: rows[index] ? rowHeight(rows[index]) : ROW_HEIGHT,
          offset: offsets[index] ?? 0,
          index
        })}
        ListEmptyComponent={
          <AppText variant="caption" muted style={styles.empty}>
            No matches
          </AppText>
        }
        renderItem={({ item: row }) => {
          if (row.kind === 'header') {
            return (
              <View style={styles.groupHeader}>
                <AppText variant="label" tone="faint" numberOfLines={1}>
                  {row.label}
                </AppText>
              </View>
            );
          }
          if (row.kind === 'create') {
            return (
              <Pressable
                style={({ pressed }) => [styles.option, pressed && styles.optionPressed]}
                onPress={() => onCreate?.(row.name)}
              >
                <View style={styles.createIcon}>
                  <Plus size={15} color={colors.primary} />
                </View>
                <AppText numberOfLines={1} style={[styles.grow, styles.createText]}>
                  Create “{row.name}”
                </AppText>
              </Pressable>
            );
          }
          if (row.kind === 'clear') {
            return (
              <Pressable
                style={({ pressed }) => [styles.option, !selectedId && styles.optionSelected, pressed && styles.optionPressed]}
                onPress={() => onSelect(null)}
              >
                <AppText muted style={styles.grow}>
                  {clearLabel}
                </AppText>
                {selectedId ? null : <Check size={16} color={colors.primary} />}
              </Pressable>
            );
          }
          const { option } = row;
          const selected = option.id === selectedId;
          return (
            <Pressable
              style={({ pressed }) => [styles.option, selected && styles.optionSelected, pressed && styles.optionPressed]}
              onPress={() => onSelect(option.id)}
            >
              {option.color ? <View style={[styles.dot, { backgroundColor: option.color }]} /> : null}
              <View style={styles.grow}>
                <AppText numberOfLines={1} style={selected ? styles.optionSelectedText : undefined}>
                  {option.label}
                </AppText>
                {option.sublabel ? (
                  <AppText variant="caption" muted numberOfLines={1}>
                    {option.sublabel}
                  </AppText>
                ) : null}
              </View>
              {selected ? <Check size={16} color={colors.primary} /> : null}
            </Pressable>
          );
        }}
      />
    </View>
  );
}

/**
 * Searchable option list rendered as an overlay sheet. Use this variant inside
 * an existing Modal (nested Modals misbehave on Android); use PickerModal when
 * there is no surrounding modal.
 */
export function PickerOverlay({ title, onClose, ...props }: PickerListProps) {
  return (
    <SheetOverlay title={title} onClose={onClose} size="tall">
      <PickerBody {...props} />
    </SheetOverlay>
  );
}

/** Standalone picker for screens that are not already inside a Modal. */
export function PickerModal({ visible, title, onClose, ...props }: PickerListProps & { visible: boolean }) {
  return (
    <Sheet visible={visible} title={title} onClose={onClose} size="tall">
      {/* remount per open so search and scroll start fresh */}
      {visible ? <PickerBody {...props} /> : null}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  field: {
    gap: spacing.sm
  },
  fieldButton: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 48,
    borderRadius: radii.md,
    paddingHorizontal: spacing.lg,
    backgroundColor: colors.surfaceStrong
  },
  fieldValue: {
    flex: 1
  },
  fieldPlaceholder: {
    color: colors.faint
  },
  pressed: {
    opacity: 0.7
  },
  grow: {
    flex: 1,
    minWidth: 0
  },
  body: {
    flex: 1,
    gap: spacing.sm
  },
  list: {
    flex: 1
  },
  searchBox: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 44,
    borderRadius: radii.full,
    paddingHorizontal: spacing.lg,
    backgroundColor: colors.surfaceStrong
  },
  searchInput: {
    flex: 1,
    color: colors.text,
    fontSize: 15,
    paddingVertical: 0
  },
  groupHeader: {
    height: HEADER_HEIGHT,
    justifyContent: 'flex-end',
    paddingHorizontal: spacing.md,
    paddingBottom: spacing.xs
  },
  option: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    height: ROW_HEIGHT,
    paddingHorizontal: spacing.md,
    borderRadius: radii.md
  },
  optionSelected: {
    backgroundColor: colors.primaryMuted
  },
  optionPressed: {
    backgroundColor: colors.surfaceStrong
  },
  optionSelectedText: {
    color: colors.primary,
    fontWeight: '600'
  },
  createIcon: {
    width: 26,
    height: 26,
    borderRadius: 13,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryMuted
  },
  createText: {
    color: colors.primary,
    fontWeight: '600'
  },
  dot: {
    width: 10,
    height: 10,
    borderRadius: 5
  },
  empty: {
    textAlign: 'center',
    paddingVertical: spacing.xl
  }
});
