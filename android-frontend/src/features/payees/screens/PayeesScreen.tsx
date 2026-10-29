import { useEffect, useMemo, useState } from 'react';
import { Alert, FlatList, Modal, Pressable, RefreshControl, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import {
  ArrowRight,
  Check,
  ChevronRight,
  GitMerge,
  ListFilter,
  Pencil,
  Plus,
  ReceiptText,
  Search,
  Trash2,
  TriangleAlert,
  Users,
  X
} from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Segmented } from '../../../components/charts';
import { EmptyState } from '../../../components/EmptyState';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { PickerModal, PickerOverlay, type PickerOption } from '../../../components/Picker';
import { Screen } from '../../../components/Screen';
import { Sheet, SheetOverlay } from '../../../components/Sheet';
import type { AppTabParamList } from '../../../navigation/types';
import { colors, radii, spacing, tabBarClearance } from '../../../theme';
import { formatCurrency } from '../../../utils/date';
import { findDuplicateGroups, formatLastUsed, hasWildcard, normalizePayeeName, testRules } from '../payeeUtils';
import {
  createPayee,
  deletePayeeRule,
  deletePayees,
  mergePayees,
  refreshPayeeData,
  renamePayee,
  savePayeeRule,
  type PayeeRuleInput
} from '../store/payeeSlice';
import type { Payee, PayeeMatchType, PayeeRule, PayeeStats } from '../types';

type Filter = 'all' | 'rules' | 'norules' | 'unused' | 'duplicates' | 'transfers';
type Sort = 'name' | 'used' | 'recent' | 'spend';

const FILTERS: { id: Filter; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'rules', label: 'With rules' },
  { id: 'norules', label: 'No rules' },
  { id: 'unused', label: 'Unused' },
  { id: 'duplicates', label: 'Duplicates' },
  { id: 'transfers', label: 'Transfers' }
];

const SORTS = [
  { value: 'name', label: 'A–Z' },
  { value: 'used', label: 'Most used' },
  { value: 'recent', label: 'Recent' },
  { value: 'spend', label: 'Spend' }
] as const;

const errorText = (err: unknown) => (err instanceof Error ? err.message : 'Please try again.');

function useCategoryNames() {
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  return useMemo(() => {
    const names: Record<string, string> = {};
    const options: PickerOption[] = [];
    const ordered = [...groups.filter((group) => group.isSystem), ...groups.filter((group) => !group.isSystem)];
    for (const group of ordered) {
      for (const category of group.categories) {
        if (!category.id) continue;
        names[category.id] = category.name;
        if (!category.hidden && group.name !== 'Hidden') {
          options.push({ id: category.id, label: category.name, group: group.isSystem ? 'Inflow' : group.name });
        }
      }
    }
    return { names, options };
  }, [groups]);
}

// ── Rule editor ──

function RuleEditor({
  payeeId,
  rule,
  allRules,
  onSave,
  onCancel,
  onPickCategory,
  categoryId,
  categoryName,
  saving
}: {
  payeeId: string;
  rule?: PayeeRule;
  allRules: PayeeRule[];
  onSave: (input: PayeeRuleInput) => void;
  onCancel: () => void;
  onPickCategory: () => void;
  categoryId: string;
  categoryName?: string;
  saving: boolean;
}) {
  const [matchString, setMatchString] = useState(rule?.matchString ?? '');
  const [matchType, setMatchType] = useState<PayeeMatchType>(rule?.matchType ?? 'EXACT');
  const [sample, setSample] = useState('');
  const trimmed = matchString.trim();
  // match strings are unique per budget; saving one that exists moves it here
  const clash = allRules.find((other) => other.matchString === trimmed && other.id !== rule?.id && other.payeeId !== payeeId);
  const result = sample.trim()
    ? testRules(
        [
          ...allRules.filter((other) => other.id !== rule?.id),
          { ...(rule ?? { id: '__draft__', budgetId: '' }), payeeId, matchString: trimmed, matchType, categoryId } as PayeeRule
        ],
        sample.trim()
      )
    : null;

  return (
    <View style={styles.editor}>
      <TextInput
        value={matchString}
        onChangeText={setMatchString}
        placeholder="Merchant or UPI handle from the bank email"
        placeholderTextColor={colors.faint}
        autoCapitalize="none"
        autoCorrect={false}
        style={styles.input}
      />
      <Segmented
        options={[
          { value: 'EXACT', label: 'Exact' },
          { value: 'PATTERN', label: 'Pattern' }
        ]}
        value={matchType}
        onChange={setMatchType}
      />
      <AppText variant="caption" tone="faint">
        {matchType === 'EXACT'
          ? 'Matches the whole string, case-sensitive.'
          : hasWildcard(trimmed)
            ? 'Case-insensitive. % is any run of characters, _ is one character.'
            : 'No % or _ yet, so this is just a case-insensitive exact match.'}
      </AppText>
      <Pressable style={styles.selectRow} onPress={onPickCategory}>
        <AppText variant="caption" muted>
          Category
        </AppText>
        <AppText weight="medium" numberOfLines={1} style={[styles.grow, styles.right, !categoryId && styles.warnText]}>
          {categoryName ?? 'None (pipeline skips the rule)'}
        </AppText>
        <ChevronRight size={16} color={colors.faint} />
      </Pressable>
      {clash ? (
        <AppText variant="caption" tone="danger">
          “{clash.matchString}” belongs to {clash.payeeName ?? 'another payee'}; saving moves it here.
        </AppText>
      ) : null}
      <TextInput
        value={sample}
        onChangeText={setSample}
        placeholder="Test: paste a bank string"
        placeholderTextColor={colors.faint}
        autoCapitalize="none"
        autoCorrect={false}
        style={styles.input}
      />
      {result ? (
        <AppText variant="caption" tone={result.winner?.payeeId === payeeId ? 'success' : result.winner ? 'danger' : 'default'}>
          {result.winner
            ? result.winner.payeeId === payeeId
              ? `Matches${result.winner.matchString === trimmed ? ' this rule' : ` “${result.winner.matchString}”`}.`
              : `Another payee's rule wins: “${result.winner.matchString}” (${result.winner.payeeName ?? 'other payee'}).`
            : 'No rule matches.'}
        </AppText>
      ) : null}
      <View style={styles.editorActions}>
        <Button size="sm" variant="secondary" onPress={onCancel} style={styles.grow}>
          Cancel
        </Button>
        <Button
          size="sm"
          style={styles.grow}
          disabled={!trimmed || saving}
          onPress={() =>
            onSave({
              ruleId: rule?.id,
              originalPayeeId: rule?.payeeId,
              originalMatchString: rule?.matchString,
              payeeId,
              matchString: trimmed,
              matchType,
              categoryId
            })
          }
        >
          {saving ? 'Saving…' : rule ? 'Save rule' : 'Add rule'}
        </Button>
      </View>
    </View>
  );
}

// ── Payee detail ──

function PayeeDetail({
  payee,
  stats,
  rules,
  allRules,
  payees,
  onClose,
  onViewTransactions
}: {
  payee: Payee;
  stats?: PayeeStats;
  rules: PayeeRule[];
  allRules: PayeeRule[];
  payees: Payee[];
  onClose: () => void;
  onViewTransactions: () => void;
}) {
  const dispatch = useAppDispatch();
  const { names, options } = useCategoryNames();
  const isTransfer = Boolean(payee.transferAccountId);
  const [renaming, setRenaming] = useState(false);
  const [name, setName] = useState(payee.name);
  const [editing, setEditing] = useState<string | null>(null);
  const [ruleCategory, setRuleCategory] = useState('');
  const [overlay, setOverlay] = useState<'category' | 'merge' | null>(null);
  const [saving, setSaving] = useState(false);

  const count = stats?.transactionCount ?? 0;
  const topCategory = stats?.topCategoryId ? names[stats.topCategoryId] : undefined;
  const inactive = rules.filter((rule) => !rule.categoryId).length;

  const run = async (task: () => Promise<unknown>, failTitle: string) => {
    setSaving(true);
    try {
      await task();
      return true;
    } catch (err) {
      Alert.alert(failTitle, errorText(err));
      return false;
    } finally {
      setSaving(false);
    }
  };

  const startEdit = (rule?: PayeeRule) => {
    setEditing(rule?.id ?? 'new');
    setRuleCategory(rule?.categoryId ?? stats?.topCategoryId ?? '');
  };

  const submitRename = async () => {
    const next = name.trim();
    if (!payee.id || !next || next === payee.name) {
      setRenaming(false);
      setName(payee.name);
      return;
    }
    if (await run(() => dispatch(renamePayee({ id: payee.id as string, name: next })).unwrap(), "Couldn't rename")) {
      setRenaming(false);
    }
  };

  const confirmDelete = () =>
    Alert.alert(
      `Delete ${payee.name}?`,
      count
        ? `${count} transactions keep their history but lose this payee. Merging into another payee is usually better.`
        : 'This payee has no transactions.',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Delete',
          style: 'destructive',
          onPress: () =>
            void run(async () => {
              const failed = await dispatch(deletePayees([payee.id as string])).unwrap();
              if (failed.length) throw new Error('The server refused the delete.');
              onClose();
            }, "Couldn't delete payee")
        }
      ]
    );

  const mergeInto = (targetId: string) => {
    const target = payees.find((other) => other.id === targetId);
    if (!target || !payee.id) return;
    Alert.alert(
      `Merge into ${target.name}?`,
      `${payee.name}'s ${count} transactions and ${rules.length} rules move to ${target.name}, and ${payee.name} is removed.`,
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Merge',
          onPress: () =>
            void run(async () => {
              await dispatch(mergePayees({ targetId, sourceIds: [payee.id as string] })).unwrap();
              onClose();
            }, "Couldn't merge")
        }
      ]
    );
  };

  const confirmDeleteRule = (rule: PayeeRule) =>
    Alert.alert('Delete rule?', `“${rule.matchString}” will stop matching ${payee.name}.`, [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: () =>
          void run(
            () => dispatch(deletePayeeRule({ payeeId: rule.payeeId, ruleId: rule.id })).unwrap(),
            "Couldn't delete rule"
          )
      }
    ]);

  const saveRule = async (input: PayeeRuleInput) => {
    if (await run(() => dispatch(savePayeeRule(input)).unwrap(), "Couldn't save rule")) setEditing(null);
  };

  const mergeOptions = payees
    .filter((other) => other.id && other.id !== payee.id && !other.transferAccountId)
    .sort((a, b) => a.name.localeCompare(b.name))
    .map((other) => ({ id: other.id as string, label: other.name }));

  const renderEditor = (rule?: PayeeRule) => (
    <RuleEditor
      key={rule?.id ?? 'new'}
      payeeId={payee.id as string}
      rule={rule}
      allRules={allRules}
      categoryId={ruleCategory}
      categoryName={ruleCategory ? names[ruleCategory] : undefined}
      onPickCategory={() => setOverlay('category')}
      onSave={(input) => void saveRule(input)}
      onCancel={() => setEditing(null)}
      saving={saving}
    />
  );

  return (
    <Modal
      visible
      transparent
      statusBarTranslucent
      navigationBarTranslucent
      animationType="slide"
      onRequestClose={() => (overlay ? setOverlay(null) : onClose())}
    >
      <SheetOverlay onClose={onClose} size="tall">
        <ScrollView contentContainerStyle={styles.detail} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator={false}>
          <View style={styles.detailHead}>
            <InitialAvatar name={payee.name} size={52} />
            <View style={styles.grow}>
              {renaming ? (
                <View style={styles.renameRow}>
                  <TextInput
                    value={name}
                    onChangeText={setName}
                    autoFocus
                    onSubmitEditing={() => void submitRename()}
                    style={[styles.input, styles.grow]}
                  />
                  <Pressable style={styles.iconButton} onPress={() => void submitRename()}>
                    <Check size={17} color={colors.primary} />
                  </Pressable>
                </View>
              ) : (
                <Pressable style={styles.renameRow} disabled={isTransfer} onPress={() => setRenaming(true)}>
                  <AppText variant="title" numberOfLines={2} style={styles.shrink}>
                    {payee.name}
                  </AppText>
                  {!isTransfer ? <Pencil size={15} color={colors.faint} /> : null}
                </Pressable>
              )}
              <AppText variant="caption" muted>
                {isTransfer ? 'Transfer payee; it belongs to an account.' : 'Payee'}
              </AppText>
            </View>
            <Pressable style={styles.iconButton} onPress={onClose} accessibilityLabel="Close">
              <X size={18} color={colors.muted} />
            </Pressable>
          </View>

          <View style={styles.statGrid}>
            {[
              ['Transactions', count.toLocaleString('en-IN')],
              ['Spent', formatCurrency(stats?.totalOutflow ?? 0)],
              ...((stats?.totalInflow ?? 0) > 0 ? [['Received', formatCurrency(stats?.totalInflow ?? 0)]] : []),
              ['Last used', formatLastUsed(stats?.lastUsed) ?? '—'],
              ['Usually', topCategory ?? '—']
            ].map(([label, value]) => (
              <View key={label} style={styles.statCell}>
                <AppText variant="label" tone="faint">
                  {label}
                </AppText>
                <AppText weight="semibold" numberOfLines={1} tabular>
                  {value}
                </AppText>
              </View>
            ))}
          </View>

          <View style={styles.actions}>
            <Button size="sm" variant="secondary" style={styles.grow} disabled={!count} onPress={onViewTransactions}>
              <View style={styles.inline}>
                <ReceiptText size={15} color={colors.text} />
                <AppText variant="caption" weight="semibold">
                  Transactions
                </AppText>
              </View>
            </Button>
            {!isTransfer ? (
              <>
                <Button size="sm" variant="secondary" style={styles.grow} onPress={() => setOverlay('merge')}>
                  <View style={styles.inline}>
                    <GitMerge size={15} color={colors.text} />
                    <AppText variant="caption" weight="semibold">
                      Merge into…
                    </AppText>
                  </View>
                </Button>
                <Pressable style={styles.dangerButton} onPress={confirmDelete} accessibilityLabel="Delete payee">
                  <Trash2 size={17} color={colors.danger} />
                </Pressable>
              </>
            ) : null}
          </View>

          {!isTransfer ? (
            <View style={styles.rulesSection}>
              <View style={styles.inline}>
                <AppText variant="heading" style={styles.grow}>
                  Rules · {rules.length}
                </AppText>
                {editing !== 'new' ? (
                  <Button size="sm" onPress={() => startEdit()}>
                    Add rule
                  </Button>
                ) : null}
              </View>
              <AppText variant="caption" muted>
                When a bank email's merchant or UPI handle matches, the transaction gets this payee and category without
                AI.
              </AppText>
              {inactive ? (
                <View style={styles.notice}>
                  <TriangleAlert size={14} color={colors.warning} />
                  <AppText variant="caption" style={[styles.grow, styles.warnText]}>
                    {inactive === 1 ? '1 rule has' : `${inactive} rules have`} no category, so the pipeline skips{' '}
                    {inactive === 1 ? 'it' : 'them'}.
                  </AppText>
                </View>
              ) : null}
              {editing === 'new' ? renderEditor() : null}
              {rules.map((rule) =>
                editing === rule.id ? (
                  renderEditor(rule)
                ) : (
                  <View key={rule.id} style={styles.ruleCard}>
                    <View style={styles.grow}>
                      <View style={styles.inline}>
                        <View style={[styles.typeBadge, rule.matchType === 'PATTERN' && styles.typePattern]}>
                          <AppText variant="caption" weight="semibold" style={styles.typeText}>
                            {rule.matchType === 'PATTERN' ? 'Pattern' : 'Exact'}
                          </AppText>
                        </View>
                        <AppText weight="medium" numberOfLines={1} style={[styles.shrink, styles.mono]}>
                          {rule.matchString}
                        </AppText>
                      </View>
                      <View style={styles.inline}>
                        <ArrowRight size={12} color={colors.faint} />
                        <AppText variant="caption" tone={rule.categoryId ? 'default' : 'danger'} numberOfLines={1}>
                          {rule.categoryId ? rule.categoryName ?? names[rule.categoryId] ?? 'Category' : 'No category'}
                        </AppText>
                      </View>
                    </View>
                    <Pressable style={styles.iconButton} onPress={() => startEdit(rule)} accessibilityLabel="Edit rule">
                      <Pencil size={15} color={colors.muted} />
                    </Pressable>
                    <Pressable style={styles.iconButton} onPress={() => confirmDeleteRule(rule)} accessibilityLabel="Delete rule">
                      <Trash2 size={15} color={colors.danger} />
                    </Pressable>
                  </View>
                )
              )}
              {!rules.length && editing !== 'new' ? (
                <AppText variant="caption" tone="faint">
                  No rules yet. Add the merchant name or UPI handle from this payee's bank emails.
                </AppText>
              ) : null}
            </View>
          ) : null}
        </ScrollView>
      </SheetOverlay>

      {overlay === 'category' ? (
        <PickerOverlay
          title="Rule category"
          options={options}
          selectedId={ruleCategory || null}
          clearLabel="No category"
          searchPlaceholder="Search categories"
          onSelect={(id) => {
            setRuleCategory(id ?? '');
            setOverlay(null);
          }}
          onClose={() => setOverlay(null)}
        />
      ) : null}
      {overlay === 'merge' ? (
        <PickerOverlay
          title={`Merge ${payee.name} into`}
          options={mergeOptions}
          searchPlaceholder="Search payees"
          autoFocusSearch
          onSelect={(id) => {
            setOverlay(null);
            if (id) mergeInto(id);
          }}
          onClose={() => setOverlay(null)}
        />
      ) : null}
    </Modal>
  );
}

// ── New payee ──

function NewPayeeSheet({
  visible,
  payees,
  onClose,
  onOpen
}: {
  visible: boolean;
  payees: Payee[];
  onClose: () => void;
  onOpen: (id: string) => void;
}) {
  const dispatch = useAppDispatch();
  const [name, setName] = useState('');
  const [saving, setSaving] = useState(false);
  const key = normalizePayeeName(name);
  const similar = key.length >= 3 ? payees.filter((payee) => payee.id && normalizePayeeName(payee.name) === key) : [];

  useEffect(() => {
    if (visible) setName('');
  }, [visible]);

  const create = async () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    setSaving(true);
    try {
      const created = await dispatch(createPayee({ name: trimmed })).unwrap();
      dispatch(refreshPayeeData());
      onClose();
      if (created.id) onOpen(created.id);
    } catch (err) {
      Alert.alert("Couldn't add payee", errorText(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet
      visible={visible}
      title="New payee"
      onClose={onClose}
      footer={
        <Button disabled={!name.trim() || saving} onPress={() => void create()}>
          {saving ? 'Adding…' : 'Add payee'}
        </Button>
      }
    >
      <View style={styles.newBody}>
        <TextInput
          value={name}
          onChangeText={setName}
          autoFocus
          placeholder="Payee name"
          placeholderTextColor={colors.faint}
          style={styles.input}
          onSubmitEditing={() => void create()}
        />
        {similar.map((payee) => (
          <Pressable key={payee.id} style={styles.notice} onPress={() => payee.id && (onClose(), onOpen(payee.id))}>
            <TriangleAlert size={14} color={colors.warning} />
            <AppText variant="caption" style={[styles.grow, styles.warnText]}>
              “{payee.name}” looks like the same payee. Open it instead?
            </AppText>
          </Pressable>
        ))}
      </View>
    </Sheet>
  );
}

// ── Screen ──

export function PayeesScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation<BottomTabNavigationProp<AppTabParamList>>();
  const { allPayees, statsById, rules, loading } = useAppSelector((state) => state.payees);
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<Filter>('all');
  const [sort, setSort] = useState<Sort>('name');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [showSort, setShowSort] = useState(false);
  const [mergeGroup, setMergeGroup] = useState<{ ids: string[]; target: string } | null>(null);

  useEffect(() => {
    dispatch(refreshPayeeData());
  }, [dispatch]);

  const payees = useMemo(() => allPayees.filter((payee) => payee.id && !payee.deleted), [allPayees]);
  const rulesByPayee = useMemo(() => {
    const grouped: Record<string, PayeeRule[]> = {};
    for (const rule of rules) (grouped[rule.payeeId] ??= []).push(rule);
    for (const list of Object.values(grouped)) list.sort((a, b) => a.matchString.localeCompare(b.matchString));
    return grouped;
  }, [rules]);
  const duplicateGroups = useMemo(() => findDuplicateGroups(payees, statsById), [payees, statsById]);

  const counts = useMemo(() => {
    const regular = payees.filter((payee) => !payee.transferAccountId);
    const stat = (payee: Payee) => statsById[payee.id as string];
    return {
      all: regular.length,
      rules: regular.filter((payee) => (stat(payee)?.ruleCount ?? 0) > 0).length,
      norules: regular.filter((payee) => !(stat(payee)?.ruleCount ?? 0)).length,
      unused: regular.filter((payee) => !(stat(payee)?.transactionCount ?? 0)).length,
      duplicates: duplicateGroups.reduce((sum, group) => sum + group.payees.length, 0),
      transfers: payees.length - regular.length
    } satisfies Record<Filter, number>;
  }, [payees, statsById, duplicateGroups]);

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    const stat = (payee: Payee) => statsById[payee.id as string];
    const keep = (payee: Payee) => {
      switch (filter) {
        case 'transfers':
          return Boolean(payee.transferAccountId);
        case 'rules':
          return !payee.transferAccountId && (stat(payee)?.ruleCount ?? 0) > 0;
        case 'norules':
          return !payee.transferAccountId && !(stat(payee)?.ruleCount ?? 0);
        case 'unused':
          return !payee.transferAccountId && !(stat(payee)?.transactionCount ?? 0);
        default:
          return !payee.transferAccountId;
      }
    };
    const matches = (payee: Payee) =>
      !q ||
      payee.name.toLowerCase().includes(q) ||
      (rulesByPayee[payee.id as string] ?? []).some((rule) => rule.matchString.toLowerCase().includes(q));
    const compare = (a: Payee, b: Payee) => {
      const sa = stat(a);
      const sb = stat(b);
      switch (sort) {
        case 'used':
          return (sb?.transactionCount ?? 0) - (sa?.transactionCount ?? 0) || a.name.localeCompare(b.name);
        case 'recent':
          return (sb?.lastUsed ?? '').localeCompare(sa?.lastUsed ?? '') || a.name.localeCompare(b.name);
        case 'spend':
          return (sb?.totalOutflow ?? 0) - (sa?.totalOutflow ?? 0) || a.name.localeCompare(b.name);
        default:
          return a.name.localeCompare(b.name);
      }
    };
    return payees.filter((payee) => keep(payee) && matches(payee)).sort(compare);
  }, [payees, statsById, rulesByPayee, query, filter, sort]);

  const selected = payees.find((payee) => payee.id === selectedId) ?? null;

  const runMerge = async (targetId: string, ids: string[]) => {
    try {
      const result = await dispatch(mergePayees({ targetId, sourceIds: ids.filter((id) => id !== targetId) })).unwrap();
      Alert.alert('Merged', `${result.mergedPayees} payees and ${result.transactions} transactions moved.`);
      setMergeGroup(null);
    } catch (err) {
      Alert.alert("Couldn't merge", errorText(err));
    }
  };

  const renderRow = ({ item }: { item: Payee }) => {
    const stats = statsById[item.id as string];
    const used = stats?.transactionCount ?? 0;
    const ruleCount = stats?.ruleCount ?? 0;
    const last = formatLastUsed(stats?.lastUsed);
    return (
      <Pressable
        android_ripple={{ color: colors.surfaceTertiary }}
        style={({ pressed }) => [styles.row, pressed && styles.rowPressed]}
        onPress={() => setSelectedId(item.id as string)}
      >
        <InitialAvatar name={item.name} size={38} />
        <View style={styles.grow}>
          <AppText weight="medium" numberOfLines={1}>
            {item.name}
          </AppText>
          <AppText variant="caption" muted numberOfLines={1}>
            {used ? `${used} txn${used === 1 ? '' : 's'}` : 'Unused'}
            {last ? ` · ${last}` : ''}
            {ruleCount ? ` · ${ruleCount} rule${ruleCount === 1 ? '' : 's'}` : ''}
          </AppText>
        </View>
        {stats?.totalOutflow ? (
          <AppText variant="caption" weight="semibold" tabular>
            {formatCurrency(stats.totalOutflow)}
          </AppText>
        ) : null}
        <ChevronRight size={16} color={colors.faint} />
      </Pressable>
    );
  };

  return (
    <Screen scroll={false} style={styles.screen}>
      <View style={styles.header}>
        <View style={styles.grow}>
          <AppText variant="title">Payees</AppText>
          <AppText variant="caption" muted>
            {counts.all} payees · {rules.length} rules
          </AppText>
        </View>
        <Pressable
          accessibilityLabel="Sort"
          style={({ pressed }) => [styles.roundButton, showSort && styles.roundButtonActive, pressed && styles.pressed]}
          onPress={() => setShowSort((open) => !open)}
        >
          <ListFilter size={19} color={showSort ? colors.primary : colors.text} />
        </Pressable>
        <Pressable
          accessibilityLabel="Add payee"
          style={({ pressed }) => [styles.addButton, pressed && styles.pressed]}
          onPress={() => setCreating(true)}
        >
          <Plus size={22} color={colors.onPrimary} />
        </Pressable>
      </View>

      <View style={styles.searchBox}>
        <Search size={17} color={colors.faint} />
        <TextInput
          value={query}
          onChangeText={setQuery}
          placeholder="Search payees and rule strings"
          placeholderTextColor={colors.faint}
          style={styles.searchInput}
        />
        {query ? (
          <Pressable hitSlop={8} onPress={() => setQuery('')}>
            <X size={16} color={colors.faint} />
          </Pressable>
        ) : null}
      </View>

      {showSort ? <Segmented options={SORTS} value={sort} onChange={setSort} /> : null}

      <View>
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.chips}>
          {FILTERS.map((option) => {
            const active = filter === option.id;
            return (
              <Pressable
                key={option.id}
                style={[styles.chip, active && styles.chipActive]}
                onPress={() => setFilter(option.id)}
              >
                <AppText variant="caption" weight="semibold" style={active ? styles.chipActiveText : styles.chipText}>
                  {option.label} {counts[option.id]}
                </AppText>
              </Pressable>
            );
          })}
        </ScrollView>
      </View>

      {filter === 'duplicates' ? (
        <FlatList
          data={duplicateGroups}
          keyExtractor={(group) => group.key}
          contentContainerStyle={styles.list}
          ListHeaderComponent={
            <AppText variant="caption" muted style={styles.listNote}>
              Payees whose names look like the same merchant. Merging moves transactions and rules to the one you keep.
            </AppText>
          }
          ListEmptyComponent={
            <EmptyState icon={<Users size={26} color={colors.primary} />} title="No duplicates" body="Every payee name looks distinct." />
          }
          renderItem={({ item: group }) => (
            <View style={styles.dupCard}>
              {group.payees.map((payee) => (
                <Pressable key={payee.id} style={styles.dupRow} onPress={() => setSelectedId(payee.id as string)}>
                  <AppText numberOfLines={1} style={styles.grow}>
                    {payee.name}
                  </AppText>
                  <AppText variant="caption" tone="faint">
                    {statsById[payee.id as string]?.transactionCount ?? 0} txns
                  </AppText>
                </Pressable>
              ))}
              <Button
                size="sm"
                onPress={() =>
                  setMergeGroup({ ids: group.payees.map((payee) => payee.id as string), target: group.suggestedTargetId })
                }
              >
                Merge…
              </Button>
            </View>
          )}
        />
      ) : (
        <FlatList
          data={rows}
          keyExtractor={(item) => item.id as string}
          renderItem={renderRow}
          contentContainerStyle={styles.list}
          keyboardShouldPersistTaps="handled"
          refreshControl={
            <RefreshControl
              refreshing={loading === 'pending'}
              onRefresh={() => dispatch(refreshPayeeData())}
              tintColor={colors.primary}
            />
          }
          ListHeaderComponent={
            counts.duplicates && filter === 'all' && !query ? (
              <Pressable style={styles.banner} onPress={() => setFilter('duplicates')}>
                <GitMerge size={16} color={colors.primary} />
                <AppText variant="caption" weight="semibold" tone="primary" style={styles.grow}>
                  {duplicateGroups.length} possible duplicate {duplicateGroups.length === 1 ? 'group' : 'groups'}
                </AppText>
                <ArrowRight size={14} color={colors.primary} />
              </Pressable>
            ) : null
          }
          ListEmptyComponent={
            <EmptyState
              icon={<Users size={26} color={colors.primary} />}
              title={query ? 'No matches' : 'Nothing here'}
              body={query ? 'No payee or rule string matches the search.' : 'No payees in this view.'}
            />
          }
        />
      )}

      {selected ? (
        <PayeeDetail
          key={selected.id}
          payee={selected}
          stats={statsById[selected.id as string]}
          rules={rulesByPayee[selected.id as string] ?? []}
          allRules={rules}
          payees={payees}
          onClose={() => setSelectedId(null)}
          onViewTransactions={() => {
            setSelectedId(null);
            navigation.navigate('Transactions', { payeeId: selected.id, payeeName: selected.name });
          }}
        />
      ) : null}

      <NewPayeeSheet visible={creating} payees={payees} onClose={() => setCreating(false)} onOpen={setSelectedId} />

      <PickerModal
        visible={mergeGroup !== null}
        title="Keep which payee?"
        options={(mergeGroup?.ids ?? []).map((id) => ({
          id,
          label: payees.find((payee) => payee.id === id)?.name ?? id,
          sublabel: `${statsById[id]?.transactionCount ?? 0} transactions${id === mergeGroup?.target ? ' · suggested' : ''}`
        }))}
        selectedId={mergeGroup?.target}
        onClose={() => setMergeGroup(null)}
        onSelect={(id) => {
          if (!id || !mergeGroup) return;
          const others = mergeGroup.ids.filter((other) => other !== id);
          Alert.alert(
            `Keep ${payees.find((payee) => payee.id === id)?.name}?`,
            `${others.length} other ${others.length === 1 ? 'payee is' : 'payees are'} merged into it.`,
            [
              { text: 'Cancel', style: 'cancel' },
              { text: 'Merge', onPress: () => void runMerge(id, mergeGroup.ids) }
            ]
          );
        }}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.md
  },
  grow: {
    flex: 1,
    minWidth: 0
  },
  shrink: {
    flexShrink: 1
  },
  right: {
    textAlign: 'right'
  },
  inline: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  pressed: {
    opacity: 0.7
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  roundButton: {
    width: 44,
    height: 44,
    borderRadius: 22,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surface
  },
  roundButtonActive: {
    backgroundColor: colors.primaryMuted
  },
  addButton: {
    width: 44,
    height: 44,
    borderRadius: 22,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primary
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
  chips: {
    gap: spacing.sm
  },
  chip: {
    minHeight: 32,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: radii.full,
    backgroundColor: colors.surface
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
  list: {
    gap: spacing.xs,
    paddingBottom: tabBarClearance
  },
  listNote: {
    marginBottom: spacing.sm
  },
  banner: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderRadius: radii.md,
    backgroundColor: colors.primaryMuted,
    padding: spacing.md,
    marginBottom: spacing.sm
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    backgroundColor: colors.surface,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.md,
    overflow: 'hidden'
  },
  rowPressed: {
    backgroundColor: colors.surfaceStrong
  },
  dupCard: {
    gap: spacing.sm,
    backgroundColor: colors.surface,
    borderRadius: radii.lg,
    padding: spacing.md,
    marginBottom: spacing.sm
  },
  dupRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    paddingVertical: spacing.xs
  },
  detail: {
    gap: spacing.lg,
    paddingBottom: spacing.xl
  },
  detailHead: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  renameRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  iconButton: {
    width: 36,
    height: 36,
    borderRadius: 18,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  statGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    rowGap: spacing.md,
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.lg,
    padding: spacing.lg
  },
  statCell: {
    width: '50%',
    gap: 2,
    paddingRight: spacing.sm
  },
  actions: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  dangerButton: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.dangerMuted
  },
  rulesSection: {
    gap: spacing.md
  },
  notice: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderRadius: radii.md,
    backgroundColor: colors.warningMuted,
    padding: spacing.md
  },
  warnText: {
    color: colors.warning
  },
  ruleCard: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    padding: spacing.md
  },
  typeBadge: {
    borderRadius: radii.sm,
    paddingHorizontal: 6,
    paddingVertical: 1,
    backgroundColor: colors.primaryMuted
  },
  typePattern: {
    backgroundColor: 'rgba(144, 133, 233, 0.18)'
  },
  typeText: {
    fontSize: 11,
    color: colors.text
  },
  mono: {
    fontFamily: 'monospace'
  },
  editor: {
    gap: spacing.sm,
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: colors.primary,
    padding: spacing.md
  },
  editorActions: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  selectRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 46,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md,
    backgroundColor: colors.surfaceStrong
  },
  input: {
    minHeight: 46,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md,
    color: colors.text,
    backgroundColor: colors.surfaceStrong,
    fontSize: 15
  },
  newBody: {
    gap: spacing.md,
    paddingBottom: spacing.sm
  }
});
