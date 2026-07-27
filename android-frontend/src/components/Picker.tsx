import { useMemo, useState } from 'react';
import { FlatList, Modal, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { Check, ChevronDown, Search, X } from 'lucide-react-native';
import { AppText } from './AppText';
import { colors, radii, spacing } from '../theme';

export type PickerOption = {
  id: string;
  label: string;
  sublabel?: string;
  color?: string;
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
  clearLabel?: string;
};

/**
 * Searchable option list rendered as an overlay. Use this variant inside an
 * existing Modal (nested Modals misbehave on Android); use PickerModal when
 * there is no surrounding modal.
 */
export function PickerOverlay({
  title,
  options,
  selectedId,
  onSelect,
  onClose,
  searchPlaceholder = 'Search',
  clearLabel
}: PickerListProps) {
  const [query, setQuery] = useState('');

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return options;
    return options.filter(
      (option) => option.label.toLowerCase().includes(q) || (option.sublabel ?? '').toLowerCase().includes(q)
    );
  }, [options, query]);

  return (
    <View style={StyleSheet.absoluteFill}>
      <Pressable style={styles.overlayBackdrop} onPress={onClose} />
      <View style={styles.sheet}>
        <View style={styles.sheetHandle} />
        <View style={styles.sheetHeader}>
          <AppText variant="heading" style={styles.grow}>
            {title}
          </AppText>
          <Pressable style={({ pressed }) => [styles.closeButton, pressed && styles.pressed]} onPress={onClose}>
            <X size={18} color={colors.muted} />
          </Pressable>
        </View>

        <View style={styles.searchBox}>
          <Search size={16} color={colors.faint} />
          <TextInput
            value={query}
            onChangeText={setQuery}
            placeholder={searchPlaceholder}
            placeholderTextColor={colors.faint}
            style={styles.searchInput}
            autoCorrect={false}
          />
        </View>

        <FlatList
          data={filtered}
          keyExtractor={(item) => item.id}
          keyboardShouldPersistTaps="handled"
          showsVerticalScrollIndicator={false}
          contentContainerStyle={styles.listContent}
          ListHeaderComponent={
            clearLabel ? (
              <Pressable
                style={({ pressed }) => [styles.option, pressed && styles.optionPressed]}
                onPress={() => onSelect(null)}
              >
                <AppText muted style={styles.grow}>
                  {clearLabel}
                </AppText>
                {selectedId ? null : <Check size={16} color={colors.primary} />}
              </Pressable>
            ) : null
          }
          ListEmptyComponent={
            <AppText variant="caption" muted style={styles.empty}>
              No matches
            </AppText>
          }
          renderItem={({ item }) => {
            const selected = item.id === selectedId;
            return (
              <Pressable
                style={({ pressed }) => [styles.option, pressed && styles.optionPressed]}
                onPress={() => onSelect(item.id)}
              >
                {item.color ? <View style={[styles.dot, { backgroundColor: item.color }]} /> : null}
                <View style={styles.grow}>
                  <AppText numberOfLines={1} style={selected ? styles.optionSelectedText : undefined}>
                    {item.label}
                  </AppText>
                  {item.sublabel ? (
                    <AppText variant="caption" muted numberOfLines={1}>
                      {item.sublabel}
                    </AppText>
                  ) : null}
                </View>
                {selected ? <Check size={16} color={colors.primary} /> : null}
              </Pressable>
            );
          }}
        />
      </View>
    </View>
  );
}

/** Standalone picker for screens that are not already inside a Modal. */
export function PickerModal({ visible, ...props }: PickerListProps & { visible: boolean }) {
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={props.onClose}>
      <PickerOverlay {...props} />
    </Modal>
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
  overlayBackdrop: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: 'rgba(0,0,0,0.55)'
  },
  sheet: {
    position: 'absolute',
    left: 0,
    right: 0,
    bottom: 0,
    maxHeight: '80%',
    backgroundColor: colors.surface,
    borderTopLeftRadius: radii.xl,
    borderTopRightRadius: radii.xl,
    paddingHorizontal: spacing.xl,
    paddingTop: spacing.md,
    paddingBottom: spacing.xl,
    gap: spacing.md
  },
  sheetHandle: {
    alignSelf: 'center',
    width: 36,
    height: 4,
    borderRadius: 2,
    backgroundColor: colors.surfaceTertiary
  },
  sheetHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  closeButton: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
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
  listContent: {
    paddingBottom: spacing.sm
  },
  option: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    minHeight: 50,
    paddingHorizontal: spacing.md,
    borderRadius: radii.md
  },
  optionPressed: {
    backgroundColor: colors.surfaceStrong
  },
  optionSelectedText: {
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
