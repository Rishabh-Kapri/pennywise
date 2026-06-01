import { useMemo, useState } from 'react';
import { FlatList, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { CirclePlus, Search } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { SectionHeader } from '../../../components/SectionHeader';
import { createPayee, fetchAllPayees } from '../store/payeeSlice';
import { colors, radii, spacing } from '../../../theme';

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
      <SectionHeader title="Payees" subtitle="Manage merchants and transfer payees used by transactions." />

      <Card style={styles.createCard}>
        <View style={styles.createRow}>
          <TextInput value={newPayee} onChangeText={setNewPayee} placeholder="New payee name" style={styles.input} />
          <Pressable style={styles.iconButton} onPress={() => void create()}>
            <CirclePlus size={22} color="#fff" />
          </Pressable>
        </View>
      </Card>

      <View style={styles.searchBox}>
        <Search size={18} color={colors.muted} />
        <TextInput value={query} onChangeText={setQuery} placeholder="Search payees" style={styles.searchInput} />
      </View>

      <FlatList
        data={filtered}
        keyExtractor={(item) => item.id ?? item.name}
        contentContainerStyle={styles.listContent}
        showsVerticalScrollIndicator={false}
        renderItem={({ item }) => (
          <Card style={styles.payeeCard}>
            <View style={styles.initial}>
              <AppText weight="bold" style={styles.initialText}>{item.name.charAt(0).toUpperCase()}</AppText>
            </View>
            <View style={styles.payeeMain}>
              <AppText weight="semibold">{item.name}</AppText>
              <AppText muted>{item.transferAccountId ? 'Transfer payee' : 'Merchant payee'}</AppText>
            </View>
            <Button variant="ghost">Rules</Button>
          </Card>
        )}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.md
  },
  createCard: {
    padding: spacing.md
  },
  createRow: {
    flexDirection: 'row',
    gap: spacing.md,
    alignItems: 'center'
  },
  input: {
    flex: 1,
    minHeight: 44,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.border,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.md,
    color: colors.text,
    backgroundColor: colors.surface
  },
  iconButton: {
    width: 44,
    height: 44,
    borderRadius: radii.sm,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primary
  },
  searchBox: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 46,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.border,
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md
  },
  searchInput: {
    flex: 1,
    color: colors.text
  },
  listContent: {
    gap: spacing.md,
    paddingBottom: spacing.xl
  },
  payeeCard: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    padding: spacing.md
  },
  initial: {
    width: 42,
    height: 42,
    borderRadius: 21,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryLight
  },
  initialText: {
    color: colors.primary
  },
  payeeMain: {
    flex: 1
  }
});
