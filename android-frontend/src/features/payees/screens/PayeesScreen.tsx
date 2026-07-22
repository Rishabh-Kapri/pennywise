import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, FlatList, Modal, Pressable, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import { Pencil, Plus, Search, Tags, Trash2, X } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { EmptyState } from '../../../components/EmptyState';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { LoadingStateView } from '../../../components/LoadingStateView';
import { createPayee, fetchAllPayees } from '../store/payeeSlice';
import { apiClient } from '../../../utils/api';
import { colors, radii, spacing, tabBarClearance } from '../../../theme';
import type { Payee, PayeeRule } from '../types';

type RuleDraft = {
  id?: string;
  matchString: string;
  matchType: 'EXACT' | 'PATTERN';
  categoryId: string | null;
};

const emptyRuleDraft: RuleDraft = { matchString: '', matchType: 'EXACT', categoryId: null };

function RulesSheet({ payee, onClose }: { payee: Payee; onClose: () => void }) {
  const categories = useAppSelector((state) => state.categories.allCategoryGroups.flatMap((group) => group.categories));
  const [rules, setRules] = useState<PayeeRule[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [ruleDraft, setRuleDraft] = useState<RuleDraft>(emptyRuleDraft);
  const [isSaving, setIsSaving] = useState(false);
  const payeeId = payee.id as string;

  const loadRules = useCallback(() => {
    setError(null);
    apiClient
      .get<PayeeRule[]>(`payees/${payeeId}/rules`)
      .then((data) => setRules(data ?? []))
      .catch((err: unknown) => {
        setRules([]);
        setError(err instanceof Error ? err.message : 'Failed to load payee rules');
      });
  }, [payeeId]);

  useEffect(loadRules, [loadRules]);

  const saveRule = async () => {
    const matchString = ruleDraft.matchString.trim();
    if (!matchString || isSaving) return;
    setIsSaving(true);
    const payload = {
      matchString,
      matchType: ruleDraft.matchType,
      categoryId: ruleDraft.categoryId || undefined
    };
    try {
      if (ruleDraft.id) await apiClient.patch<PayeeRule>(`payees/${payeeId}/rules/${ruleDraft.id}`, payload);
      else await apiClient.post<PayeeRule>(`payees/${payeeId}/rules`, payload);
      setRuleDraft(emptyRuleDraft);
      loadRules();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save rule');
    } finally {
      setIsSaving(false);
    }
  };

  const deleteRule = (rule: PayeeRule) => {
    if (!rule.id) return;
    Alert.alert('Delete rule?', `"${rule.matchString}" will no longer match this payee.`, [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: async () => {
          try {
            await apiClient.delete(`payees/${payeeId}/rules/${rule.id}`);
            if (ruleDraft.id === rule.id) setRuleDraft(emptyRuleDraft);
            loadRules();
          } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to delete rule');
          }
        }
      }
    ]);
  };

  const categoryName = (categoryId?: string | null) =>
    categories.find((category) => category.id === categoryId)?.name ?? null;

  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose}>
      <View style={styles.modalBackdrop}>
        <View style={styles.sheet}>
          <View style={styles.sheetHandle} />
          <View style={styles.rowBetween}>
            <View style={styles.sheetHeaderText}>
              <AppText variant="title" style={styles.sheetTitle}>{payee.name}</AppText>
              <AppText variant="caption" muted>
                {rules ? `${rules.length} bank match ${rules.length === 1 ? 'rule' : 'rules'}` : 'Rules'}
              </AppText>
            </View>
            <Pressable style={({ pressed }) => [styles.closeButton, pressed && styles.pressed]} onPress={onClose}>
              <X size={18} color={colors.muted} />
            </Pressable>
          </View>

          <ScrollView contentContainerStyle={styles.sheetScroll} showsVerticalScrollIndicator={false} keyboardShouldPersistTaps="handled">
            {error ? <AppText variant="caption" tone="danger">{error}</AppText> : null}

            {rules === null ? (
              <LoadingStateView label="Loading rules" />
            ) : rules.length === 0 ? (
              <AppText variant="caption" muted>
                No rules yet. Rules map bank statement text to this payee during email classification.
              </AppText>
            ) : (
              rules.map((rule, index) => (
                <View key={rule.id ?? index} style={[styles.ruleRow, index > 0 && styles.rowDivider]}>
                  <View style={styles.ruleMain}>
                    <AppText weight="medium" numberOfLines={2}>{rule.matchString}</AppText>
                    <AppText variant="caption" muted>
                      {rule.matchType === 'PATTERN' ? 'Pattern match' : 'Exact match'}
                      {categoryName(rule.categoryId) ? ` · ${categoryName(rule.categoryId)}` : ''}
                    </AppText>
                  </View>
                  <Pressable
                    accessibilityLabel="Edit rule"
                    style={({ pressed }) => [styles.ruleAction, pressed && styles.pressed]}
                    onPress={() =>
                      setRuleDraft({
                        id: rule.id,
                        matchString: rule.matchString,
                        matchType: rule.matchType || 'EXACT',
                        categoryId: rule.categoryId ?? null
                      })
                    }
                  >
                    <Pencil size={15} color={colors.muted} />
                  </Pressable>
                  <Pressable
                    accessibilityLabel="Delete rule"
                    style={({ pressed }) => [styles.ruleAction, pressed && styles.pressed]}
                    onPress={() => deleteRule(rule)}
                  >
                    <Trash2 size={15} color={colors.danger} />
                  </Pressable>
                </View>
              ))
            )}

            <View style={styles.formBlock}>
              <AppText variant="label" tone="faint">{ruleDraft.id ? 'Edit rule' : 'Add rule'}</AppText>
              <TextInput
                value={ruleDraft.matchString}
                onChangeText={(matchString) => setRuleDraft({ ...ruleDraft, matchString })}
                style={styles.input}
                placeholder="Bank statement text to match"
                placeholderTextColor={colors.faint}
              />
              <View style={styles.matchTypeRow}>
                {(['EXACT', 'PATTERN'] as const).map((matchType) => (
                  <Pressable
                    key={matchType}
                    style={[styles.choice, ruleDraft.matchType === matchType && styles.choiceSelected]}
                    onPress={() => setRuleDraft({ ...ruleDraft, matchType })}
                  >
                    <AppText
                      variant="caption"
                      weight="medium"
                      style={ruleDraft.matchType === matchType ? styles.choiceSelectedText : styles.choiceText}
                    >
                      {matchType === 'EXACT' ? 'Exact' : 'Pattern'}
                    </AppText>
                  </Pressable>
                ))}
              </View>
              <AppText variant="label" tone="faint">Default category (optional)</AppText>
              <FlatList
                horizontal
                data={categories}
                keyExtractor={(item) => item.id ?? item.name}
                showsHorizontalScrollIndicator={false}
                renderItem={({ item }) => (
                  <Pressable
                    style={[styles.choice, styles.choiceScroll, ruleDraft.categoryId === item.id && styles.choiceSelected]}
                    onPress={() => item.id && setRuleDraft({ ...ruleDraft, categoryId: ruleDraft.categoryId === item.id ? null : item.id })}
                  >
                    <AppText
                      variant="caption"
                      weight="medium"
                      style={ruleDraft.categoryId === item.id ? styles.choiceSelectedText : styles.choiceText}
                    >
                      {item.name}
                    </AppText>
                  </Pressable>
                )}
              />
              <View style={styles.formActions}>
                {ruleDraft.id ? (
                  <Button variant="secondary" size="sm" onPress={() => setRuleDraft(emptyRuleDraft)}>
                    Cancel edit
                  </Button>
                ) : null}
                <Button size="sm" disabled={isSaving || !ruleDraft.matchString.trim()} onPress={() => void saveRule()}>
                  {isSaving ? 'Saving...' : ruleDraft.id ? 'Update rule' : 'Add rule'}
                </Button>
              </View>
            </View>
          </ScrollView>
        </View>
      </View>
    </Modal>
  );
}

export function PayeesScreen() {
  const dispatch = useAppDispatch();
  const payees = useAppSelector((state) => state.payees.allPayees);
  const [query, setQuery] = useState('');
  const [newPayee, setNewPayee] = useState('');
  const [rulesPayee, setRulesPayee] = useState<Payee | null>(null);

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
            {item.id ? (
              <Button variant="ghost" size="sm" onPress={() => setRulesPayee(item)}>
                Rules
              </Button>
            ) : null}
          </View>
        )}
      />

      {rulesPayee?.id ? <RulesSheet key={rulesPayee.id} payee={rulesPayee} onClose={() => setRulesPayee(null)} /> : null}
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
  },
  rowBetween: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  modalBackdrop: {
    flex: 1,
    justifyContent: 'flex-end',
    backgroundColor: 'rgba(0,0,0,0.55)'
  },
  sheet: {
    maxHeight: '88%',
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
    backgroundColor: colors.surfaceTertiary,
    marginBottom: spacing.sm
  },
  sheetHeaderText: {
    flex: 1,
    gap: 1
  },
  sheetTitle: {
    fontSize: 20,
    lineHeight: 26
  },
  sheetScroll: {
    gap: spacing.md,
    paddingTop: spacing.sm
  },
  closeButton: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  ruleRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    paddingVertical: spacing.md
  },
  rowDivider: {
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  ruleMain: {
    flex: 1,
    gap: 2
  },
  ruleAction: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  formBlock: {
    gap: spacing.sm,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border,
    paddingTop: spacing.lg
  },
  matchTypeRow: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  choice: {
    minHeight: 36,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong
  },
  choiceScroll: {
    marginRight: spacing.sm
  },
  choiceSelected: {
    backgroundColor: colors.primaryMuted
  },
  choiceText: {
    color: colors.muted
  },
  choiceSelectedText: {
    color: colors.primary
  },
  formActions: {
    flexDirection: 'row',
    justifyContent: 'flex-end',
    gap: spacing.sm,
    marginTop: spacing.xs
  }
});
