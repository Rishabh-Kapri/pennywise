import { useMemo, useState } from 'react';
import { FlatList, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { Plus, Search, Tags } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { EmptyState } from '../../../components/EmptyState';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { createPayee, fetchAllPayees } from '../store/payeeSlice';
import { colors, radii, spacing, tabBarClearance } from '../../../theme';

export function PayeesScreen() {
  const dispatch = useAppDispatch();
  const payees = useAppSelector((state) => state.payees.allPayees);
  const [query, setQuery] = useState('');
  const [newPayee, setNewPayee] = useState('');

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return payees;
    return payees.filter((payee) => payee.name.toLowerCase().includes(q));
  }, [payees, query]);

  const create = async () => {
    const name = newPayee.trim();
    if (!name) return;
    await dispatch(createPayee({ name })).unwrap();
    setNewPayee('');
    dispatch(fetchAllPayees());
  };

  return (
    <Screen scroll={false} style={styles.screen}>
      <View style={styles.headerText}>
        <AppText variant="title">Payees</AppText>
        <AppText variant="caption" muted>{payees.length} merchants and transfer payees</AppText>
      </View>

      <View style={styles.createRow}>
        <TextInput
          value={newPayee}
          onChangeText={setNewPayee}
          placeholder="New payee name"
          placeholderTextColor={colors.faint}
          style={styles.input}
        />
        <Pressable style={({ pressed }) => [styles.addButton, pressed && styles.pressed]} onPress={() => void create()}>
          <Plus size={20} color={colors.onPrimary} />
        </Pressable>
      </View>

      <View style={styles.searchBox}>
        <Search size={17} color={colors.faint} />
        <TextInput
          value={query}
          onChangeText={setQuery}
          placeholder="Search payees"
          placeholderTextColor={colors.faint}
          style={styles.searchInput}
        />
      </View>

      <FlatList
        data={filtered}
        keyExtractor={(item) => item.id ?? item.name}
        contentContainerStyle={styles.listContent}
        showsVerticalScrollIndicator={false}
        ListEmptyComponent={
          <EmptyState
            icon={<Tags size={26} color={colors.primary} />}
            title="No payees yet"
            body="Add a payee above, or let Gmail imports create them automatically."
          />
        }
        renderItem={({ item }) => (
          <View style={styles.payeeRow}>
            <InitialAvatar name={item.name} size={42} />
            <View style={styles.payeeMain}>
              <AppText weight="medium" numberOfLines={1}>{item.name}</AppText>
              <AppText variant="caption" muted>{item.transferAccountId ? 'Transfer payee' : 'Merchant'}</AppText>
            </View>
            <Button variant="ghost" size="sm">Rules</Button>
          </View>
        )}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  headerText: {
    gap: 2
  },
  createRow: {
    flexDirection: 'row',
    gap: spacing.sm,
    alignItems: 'center'
  },
  input: {
    flex: 1,
    minHeight: 46,
    borderRadius: radii.full,
    paddingHorizontal: spacing.lg,
    color: colors.text,
    backgroundColor: colors.surfaceStrong,
    fontSize: 15
  },
  addButton: {
    width: 44,
    height: 44,
    borderRadius: 22,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primary
  },
  pressed: {
    opacity: 0.7
  },
  searchBox: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 46,
    backgroundColor: colors.surface,
    borderRadius: radii.full,
    paddingHorizontal: spacing.lg
  },
  searchInput: {
    flex: 1,
    color: colors.text,
    fontSize: 15
  },
  listContent: {
    gap: spacing.xs,
    paddingBottom: tabBarClearance
  },
  payeeRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    backgroundColor: colors.surface,
    borderRadius: radii.md,
    padding: spacing.md
  },
  payeeMain: {
    flex: 1,
    gap: 1
  }
});
