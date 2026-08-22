import { useMemo, useState } from 'react';
import { Pressable, StyleSheet, TextInput, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import { Check, ChevronLeft, Pencil, Tags as TagsIcon, Trash2, X } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { EmptyState } from '../../../components/EmptyState';
import { Screen } from '../../../components/Screen';
import { colors, radii, spacing } from '../../../theme';
import { createTag, deleteTag, selectAllTags, selectTagsError, updateTag } from '../store/tagSlice';
import type { Tag } from '../types';
import { TAG_COLORS, getAutoColor } from '../utils';

function ColorSwatchRow({ selected, onSelect }: { selected: string; onSelect: (color: string) => void }) {
  return (
    <View style={styles.swatchRow}>
      {TAG_COLORS.map((color) => (
        <Pressable
          key={color}
          accessibilityRole="radio"
          accessibilityState={{ selected: selected === color }}
          accessibilityLabel={`Tag colour ${color}`}
          onPress={() => onSelect(color)}
          style={({ pressed }) => [
            styles.swatch,
            { backgroundColor: color },
            selected === color && styles.swatchSelected,
            pressed && styles.pressed
          ]}
        />
      ))}
    </View>
  );
}

function TagChip({ name, color }: { name: string; color: string }) {
  return (
    <View style={[styles.chip, { backgroundColor: color }]}>
      <AppText variant="caption" weight="semibold" numberOfLines={1} style={styles.chipLabel}>
        {name}
      </AppText>
    </View>
  );
}

function IconButton({
  label,
  onPress,
  children
}: {
  label: string;
  onPress: () => void;
  children: React.ReactNode;
}) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      onPress={onPress}
      hitSlop={6}
      style={({ pressed }) => [styles.iconButton, pressed && styles.pressed]}
    >
      {children}
    </Pressable>
  );
}

function TagRow({ tag, isLast }: { tag: Tag; isLast: boolean }) {
  const dispatch = useAppDispatch();
  const [isEditing, setIsEditing] = useState(false);
  const [isConfirmingDelete, setIsConfirmingDelete] = useState(false);
  const [name, setName] = useState(tag.name);
  const [color, setColor] = useState(tag.color || getAutoColor(tag.name));

  const startEdit = () => {
    setName(tag.name);
    setColor(tag.color || getAutoColor(tag.name));
    setIsEditing(true);
    setIsConfirmingDelete(false);
  };

  const save = () => {
    const trimmed = name.trim();
    if (!trimmed || !tag.id) {
      setIsEditing(false);
      return;
    }
    if (trimmed !== tag.name || color !== tag.color) {
      void dispatch(updateTag({ id: tag.id, tag: { ...tag, name: trimmed, color } }));
    }
    setIsEditing(false);
  };

  if (isEditing) {
    return (
      <View style={[styles.row, styles.rowEditing, !isLast && styles.rowDivider]}>
        <View style={styles.editHeader}>
          <TagChip name={name.trim() || tag.name} color={color} />
          <View style={styles.rowActions}>
            <IconButton label="Save tag" onPress={save}>
              <Check size={17} color={colors.success} />
            </IconButton>
            <IconButton label="Cancel editing" onPress={() => setIsEditing(false)}>
              <X size={17} color={colors.muted} />
            </IconButton>
          </View>
        </View>
        <TextInput
          value={name}
          autoFocus
          onChangeText={setName}
          onSubmitEditing={save}
          returnKeyType="done"
          accessibilityLabel="Tag name"
          placeholder="Tag name"
          placeholderTextColor={colors.faint}
          style={styles.input}
        />
        <ColorSwatchRow selected={color} onSelect={setColor} />
      </View>
    );
  }

  return (
    <View style={[styles.row, !isLast && styles.rowDivider]}>
      <TagChip name={tag.name} color={tag.color || getAutoColor(tag.name)} />
      <View style={styles.rowActions}>
        {isConfirmingDelete ? (
          <>
            <Pressable
              accessibilityRole="button"
              onPress={() => tag.id && void dispatch(deleteTag(tag.id))}
              style={({ pressed }) => [styles.deleteConfirm, pressed && styles.pressed]}
            >
              <AppText variant="caption" weight="semibold" tone="danger">
                Delete?
              </AppText>
            </Pressable>
            <IconButton label="Cancel delete" onPress={() => setIsConfirmingDelete(false)}>
              <X size={17} color={colors.muted} />
            </IconButton>
          </>
        ) : (
          <>
            <IconButton label="Edit tag" onPress={startEdit}>
              <Pencil size={16} color={colors.muted} />
            </IconButton>
            <IconButton label="Delete tag" onPress={() => setIsConfirmingDelete(true)}>
              <Trash2 size={16} color={colors.muted} />
            </IconButton>
          </>
        )}
      </View>
    </View>
  );
}

export function TagsScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation();
  const allTags = useAppSelector(selectAllTags);
  const error = useAppSelector(selectTagsError);
  const [newName, setNewName] = useState('');
  const [newColor, setNewColor] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);

  const trimmedNew = newName.trim();
  const nameExists = useMemo(
    () => allTags.some((tag) => tag.name.trim().toLowerCase() === trimmedNew.toLowerCase()),
    [allTags, trimmedNew]
  );

  const handleCreate = async () => {
    if (!trimmedNew || nameExists || isCreating) return;
    setIsCreating(true);
    try {
      await dispatch(createTag({ name: trimmedNew, color: newColor ?? getAutoColor(trimmedNew) })).unwrap();
      setNewName('');
      setNewColor(null);
    } catch {
      // the slice records the failure; the banner below surfaces it
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <Screen style={styles.screen}>
      <View style={styles.headerRow}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Go back"
          style={({ pressed }) => [styles.backButton, pressed && styles.pressed]}
          onPress={() => navigation.goBack()}
        >
          <ChevronLeft size={20} color={colors.text} />
        </Pressable>
        <View style={styles.headerMain}>
          <AppText variant="title">Tags</AppText>
          <AppText variant="caption" muted>
            Create, rename, and recolor the tags used on transactions
          </AppText>
        </View>
      </View>

      <Card style={styles.createCard}>
        <TextInput
          value={newName}
          onChangeText={setNewName}
          onSubmitEditing={() => void handleCreate()}
          returnKeyType="done"
          accessibilityLabel="New tag name"
          placeholder="New tag name…"
          placeholderTextColor={colors.faint}
          style={styles.input}
        />
        <ColorSwatchRow
          selected={newColor ?? (trimmedNew ? getAutoColor(trimmedNew) : '')}
          onSelect={setNewColor}
        />
        {nameExists ? (
          <AppText variant="caption" tone="danger">
            A tag with this name already exists
          </AppText>
        ) : null}
        <Button
          size="sm"
          onPress={() => void handleCreate()}
          disabled={!trimmedNew || nameExists || isCreating}
        >
          {isCreating ? 'Adding…' : 'Add tag'}
        </Button>
      </Card>

      {error ? (
        <Card style={styles.errorCard}>
          <AppText variant="caption" tone="danger">
            {error}
          </AppText>
        </Card>
      ) : null}

      {allTags.length === 0 ? (
        <EmptyState
          icon={<TagsIcon color={colors.primary} size={24} />}
          title="No tags yet"
          body="Create one above, or type a new tag name directly on a transaction."
        />
      ) : (
        <Card style={styles.listCard}>
          {allTags.map((tag, index) => (
            <TagRow key={tag.id ?? tag.name} tag={tag} isLast={index === allTags.length - 1} />
          ))}
        </Card>
      )}
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  headerMain: {
    flex: 1,
    gap: 2
  },
  backButton: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  createCard: {
    gap: spacing.md
  },
  errorCard: {
    paddingVertical: spacing.md,
    backgroundColor: colors.dangerMuted
  },
  listCard: {
    paddingVertical: spacing.xs
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md,
    paddingVertical: spacing.md
  },
  rowEditing: {
    flexDirection: 'column',
    alignItems: 'stretch',
    gap: spacing.md
  },
  rowDivider: {
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.border
  },
  editHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  rowActions: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs
  },
  iconButton: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  deleteConfirm: {
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: radii.full,
    backgroundColor: colors.dangerMuted
  },
  chip: {
    flexShrink: 1,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.xs + 2,
    borderRadius: radii.full
  },
  chipLabel: {
    color: '#0B0B0F'
  },
  input: {
    minHeight: 44,
    paddingHorizontal: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    color: colors.text,
    fontSize: 15
  },
  swatchRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sm
  },
  swatch: {
    width: 28,
    height: 28,
    borderRadius: 14,
    borderWidth: 2,
    borderColor: 'transparent'
  },
  swatchSelected: {
    borderColor: colors.text
  },
  pressed: {
    opacity: 0.7
  }
});
