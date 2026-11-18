import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ActivityIndicator,
  Pressable,
  RefreshControl,
  SectionList,
  StyleSheet,
  TextInput,
  View
} from 'react-native';
import { useNavigation, useRoute, type RouteProp } from '@react-navigation/native';
import { ArrowLeftRight, Check, Plus, ReceiptText, Search, Sparkles, X } from 'lucide-react-native';
import { Hatch } from '../../../components/Hatch';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { EmptyState } from '../../../components/EmptyState';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { apiClient } from '../../../utils/api';
import type { PaginationResponse } from '../../../utils/constants';
import { formatCurrency, formatRelativeDay, formatSignedCurrency, getSelectedMonthInHumanFormat } from '../../../utils/date';
import { colors, radii, spacing, tabBarClearance } from '../../../theme';
import type { AppTabParamList } from '../../../navigation/types';
import type { Tag } from '../../tags/types';
import type { Transaction } from '../types';
import { TransactionStatus } from '../types';
import { TransactionEditor, createDraft, type TxnDraft } from '../components/TransactionEditor';
import { fetchAllTransactions, updateTransactionStatus } from '../store/transactionSlice';

type Filter = 'all' | 'review' | 'uncategorized';

const FILTERS: { id: Filter; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'review', label: 'Needs review' },
  { id: 'uncategorized', label: 'Uncategorized' }
];

function monthRange(month: string) {
  const [year, monthNumber] = month.split('-').map(Number);
  const lastDay = new Date(year, monthNumber, 0).getDate();
  return {
    startDate: `${month}-01`,
    endDate: `${month}-${String(lastDay).padStart(2, '0')}`
  };
}

function TagChips({ tagIds, tags }: { tagIds: string[]; tags: Tag[] }) {
  const matched = tagIds.map((id) => tags.find((tag) => tag.id === id)).filter((tag): tag is Tag => Boolean(tag));
  if (!matched.length) return null;

  return (
    <View style={styles.tagRow}>
      {matched.map((tag) => (
        <View key={tag.id} style={styles.tagChip}>
          <View style={[styles.tagDot, { backgroundColor: tag.color || colors.primary }]} />
          <AppText variant="caption" muted numberOfLines={1}>
            {tag.name}
          </AppText>
        </View>
      ))}
    </View>
  );
}

function statusText(status?: TransactionStatus): { label: string; tone: 'success' | 'faint' | 'danger' } | null {
  switch (status) {
    case TransactionStatus.APPROVED:
      return { label: 'Approved', tone: 'success' };
    case TransactionStatus.MANUAL:
      return { label: 'Manual entry', tone: 'faint' };
    case TransactionStatus.REJECTED:
      return { label: 'Rejected', tone: 'danger' };
    default:
      return null;
  }
}

/** One transaction as a card: payee and amount on top, category and status in a footer strip. */
function TransactionRow({
  txn,
  tags,
  onPress,
  onApprove
}: {
  txn: Transaction;
  tags: Tag[];
  onPress: () => void;
  onApprove: () => void;
}) {
  const amount = (txn.inflow ?? 0) - (txn.outflow ?? 0);
  const transfer = Boolean(txn.transferAccountId);
  const unapproved = txn.status === TransactionStatus.UNAPPROVED;
  const uncategorized = !txn.categoryId && !transfer;
  const name = txn.payeeName || 'Unknown payee';
  const status = statusText(txn.status);

  return (
    <Pressable style={({ pressed }) => [styles.txnCard, pressed && styles.rowPressed]} onPress={onPress}>
      <View style={styles.txnTop}>
        <View style={styles.txnTile}>
          {transfer ? <ArrowLeftRight size={18} color={colors.muted} /> : <InitialAvatar name={name} size={32} />}
        </View>
        <View style={styles.txnMain}>
          <AppText weight="semibold" numberOfLines={1}>
            {name}
          </AppText>
          <AppText variant="caption" muted numberOfLines={1}>
            {txn.note ? `${txn.accountName} · ${txn.note}` : txn.accountName}
          </AppText>
          <TagChips tagIds={txn.tagIds ?? []} tags={tags} />
        </View>
        <AppText weight="bold" tone={amount > 0 && !transfer ? 'success' : 'default'} tabular>
          {formatSignedCurrency(amount, !transfer)}
        </AppText>
      </View>
      <View style={styles.txnFoot}>
        <AppText variant="caption" muted numberOfLines={1} style={[styles.txnMain, uncategorized && styles.warnText]}>
          {transfer ? 'Transfer' : txn.categoryName ?? 'Uncategorized'}
        </AppText>
        {unapproved ? (
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Approve"
            hitSlop={8}
            style={({ pressed }) => [styles.approveButton, pressed && styles.pressed]}
            onPress={onApprove}
          >
            <Check size={12} color={colors.success} />
            <AppText variant="caption" weight="semibold" tone="success">
              Approve
            </AppText>
          </Pressable>
        ) : status ? (
          <AppText variant="caption" weight="semibold" tone={status.tone}>
            {status.label}
          </AppText>
        ) : null}
      </View>
    </Pressable>
  );
}

export function TransactionsScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation();
  const route = useRoute<RouteProp<AppTabParamList, 'Transactions'>>();
  const { transactions, nextCursor, loadingMore, loading } = useAppSelector((state) => state.transactions);
  const tags = useAppSelector((state) => state.tags.tags);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<Filter>('all');
  const [draft, setDraft] = useState<TxnDraft | null>(null);
  // bumps per open so the editor always mounts fresh (no picker left open)
  const [editorKey, setEditorKey] = useState(0);
  const [scoped, setScoped] = useState<Transaction[] | null>(null);
  const [isScopedLoading, setIsScopedLoading] = useState(false);

  const categoryId = route.params?.categoryId;
  const payeeId = route.params?.payeeId;
  // a category or payee drill-down replaces the paged list with one query
  const scopeKey = categoryId ?? payeeId;
  const scopeName = categoryId ? route.params?.categoryName ?? 'Category' : route.params?.payeeName ?? 'Payee';
  const month = route.params?.month;

  const loadScoped = useCallback(() => {
    if (!scopeKey) {
      setScoped(null);
      return;
    }
    const params = new URLSearchParams();
    if (categoryId) params.set('categoryId[]', categoryId);
    if (payeeId) params.set('payeeId[]', payeeId);
    if (month) {
      const { startDate, endDate } = monthRange(month);
      params.set('startDate', startDate);
      params.set('endDate', endDate);
    }
    params.set('limit', categoryId ? '100' : '200');
    setIsScopedLoading(true);
    apiClient
      .get<PaginationResponse<Transaction[]>>(`transactions/normalized?${params.toString()}`)
      .then((response) => setScoped(response.data ?? []))
      .catch(() => setScoped([]))
      .finally(() => setIsScopedLoading(false));
  }, [scopeKey, categoryId, payeeId, month]);

  useEffect(loadScoped, [loadScoped]);

  const clearScope = () => {
    setScoped(null);
    navigation.setParams({
      categoryId: undefined,
      categoryName: undefined,
      payeeId: undefined,
      payeeName: undefined,
      month: undefined
    } as never);
  };

  const source = scopeKey ? scoped ?? [] : transactions;

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return source.filter((txn) => {
      if (filter === 'review' && txn.status !== TransactionStatus.UNAPPROVED) return false;
      if (filter === 'uncategorized' && (txn.categoryId || txn.transferAccountId)) return false;
      if (!q) return true;
      return [txn.payeeName, txn.accountName, txn.categoryName ?? '', txn.note ?? ''].some((value) =>
        (value ?? '').toLowerCase().includes(q)
      );
    });
  }, [search, source, filter]);

  const sections = useMemo(() => {
    const grouped: { title: string; total: number; data: Transaction[] }[] = [];
    for (const txn of filtered) {
      const last = grouped[grouped.length - 1];
      const net = txn.transferAccountId ? 0 : (txn.inflow ?? 0) - (txn.outflow ?? 0);
      if (last && last.title === txn.date) {
        last.data.push(txn);
        last.total += net;
      } else {
        grouped.push({ title: txn.date, total: net, data: [txn] });
      }
    }
    return grouped;
  }, [filtered]);

  const scopedNet = useMemo(
    () => filtered.reduce((sum, txn) => (txn.transferAccountId ? sum : sum + (txn.inflow ?? 0) - (txn.outflow ?? 0)), 0),
    [filtered]
  );

  const reviewCount = useMemo(
    () => source.filter((txn) => txn.status === TransactionStatus.UNAPPROVED).length,
    [source]
  );

  const refreshLists = () => {
    dispatch(fetchAllTransactions());
    if (scopeKey) loadScoped();
  };

  const openDraft = (next: TxnDraft) => {
    setEditorKey((key) => key + 1);
    setDraft(next);
  };

  return (
    <Screen scroll={false} style={styles.screen}>
      <View style={styles.header}>
        <View style={styles.headerText}>
          <AppText variant="title">Transactions</AppText>
          <AppText variant="caption" muted>
            {scopeKey ? `${filtered.length} shown` : `${transactions.length} loaded`}
          </AppText>
        </View>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Add transaction"
          style={({ pressed }) => [styles.addButton, pressed && styles.pressed]}
          onPress={() => openDraft(createDraft())}
        >
          <Plus size={18} color={colors.onHero} />
          <AppText variant="caption" weight="semibold" style={styles.onHero}>
            Add
          </AppText>
        </Pressable>
      </View>

      {scopeKey ? (
        <View style={styles.scopeCard}>
          <Hatch color={colors.onHero} gap={9} opacity={0.05} />
          <View style={styles.scopeText}>
            <AppText variant="caption" style={styles.onHeroMuted} numberOfLines={1}>
              {month ? `${scopeName} · ${getSelectedMonthInHumanFormat(month)}` : scopeName}
            </AppText>
            <AppText variant="title" tabular style={styles.onHero} numberOfLines={1} adjustsFontSizeToFit>
              {formatCurrency(Math.abs(scopedNet))}
            </AppText>
            <AppText variant="caption" style={styles.onHeroMuted}>
              {`${scopedNet > 0 ? 'In' : 'Out'} across ${filtered.length} ${filtered.length === 1 ? 'transaction' : 'transactions'}`}
            </AppText>
          </View>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Show all transactions"
            hitSlop={8}
            style={({ pressed }) => [styles.scopeClose, pressed && styles.pressed]}
            onPress={clearScope}
          >
            <X size={16} color={colors.onHero} />
          </Pressable>
        </View>
      ) : null}

      <View style={styles.searchBox}>
        <Search size={17} color={colors.faint} />
        <TextInput
          value={search}
          onChangeText={setSearch}
          placeholder="Search payee, category, memo"
          placeholderTextColor={colors.faint}
          style={styles.searchInput}
        />
        {search ? (
          <Pressable accessibilityLabel="Clear search" hitSlop={8} onPress={() => setSearch('')}>
            <X size={16} color={colors.faint} />
          </Pressable>
        ) : null}
      </View>

      <View style={styles.filters}>
        {FILTERS.map((option) => {
          const active = filter === option.id;
          return (
            <Pressable
              key={option.id}
              style={[styles.filterChip, active && styles.filterChipActive]}
              onPress={() => setFilter(option.id)}
            >
              {option.id === 'review' ? <Sparkles size={12} color={active ? colors.background : colors.muted} /> : null}
              <AppText variant="caption" weight="semibold" style={active ? styles.filterActiveText : styles.filterText}>
                {option.label}
                {option.id === 'review' && reviewCount ? ` ${reviewCount}` : ''}
              </AppText>
            </Pressable>
          );
        })}
      </View>

      <SectionList
        sections={sections}
        keyExtractor={(item) => item.id ?? `${item.date}-${item.payeeName}-${item.amount}`}
        contentContainerStyle={styles.listContent}
        showsVerticalScrollIndicator={false}
        stickySectionHeadersEnabled={false}
        refreshControl={
          <RefreshControl
            // The list is served either from the store or from the scoped
            // category fetch, so the spinner has to track whichever is live.
            refreshing={scopeKey ? isScopedLoading : loading === 'pending'}
            onRefresh={refreshLists}
            tintColor={colors.primary}
          />
        }
        renderSectionHeader={({ section }) => (
          <View style={styles.dayHeader}>
            <AppText variant="caption" tone="faint">
              {formatRelativeDay(section.title)}
            </AppText>
            <View style={styles.dayRule} />
            {Math.round(section.total) !== 0 ? (
              <AppText variant="caption" tone="faint" tabular>
                {formatSignedCurrency(section.total, true)}
              </AppText>
            ) : null}
          </View>
        )}
        ListEmptyComponent={
          isScopedLoading ? (
            <View style={styles.scopedLoading}>
              <ActivityIndicator color={colors.primary} />
            </View>
          ) : (
            <EmptyState
              icon={<ReceiptText size={26} color={colors.primary} />}
              title={filter !== 'all' || search ? 'Nothing matches' : scopeKey ? 'No transactions' : 'No transactions yet'}
              body={
                filter === 'review'
                  ? 'Every imported transaction has been reviewed.'
                  : filter === 'uncategorized'
                    ? 'Every loaded transaction has a category.'
                    : scopeKey
                      ? 'Nothing recorded here for this selection.'
                      : 'Transactions from your accounts and Gmail imports will show up here.'
              }
            />
          )
        }
        ListFooterComponent={
          loadingMore === 'pending' ? <ActivityIndicator style={styles.footerSpinner} color={colors.primary} /> : null
        }
        onEndReached={() => {
          if (!scopeKey && nextCursor && loadingMore !== 'pending') dispatch(fetchAllTransactions({ cursor: nextCursor }));
        }}
        renderItem={({ item }) => (
          <TransactionRow
            txn={item}
            tags={tags}
            onPress={() => openDraft(createDraft(item))}
            onApprove={() =>
              item.id && dispatch(updateTransactionStatus({ id: item.id, status: TransactionStatus.APPROVED }))
            }
          />
        )}
      />

      {draft ? (
        <TransactionEditor
          key={editorKey}
          initialDraft={draft}
          onClose={() => setDraft(null)}
          onSaved={() => {
            setDraft(null);
            refreshLists();
          }}
        />
      ) : null}
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.md
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  headerText: {
    gap: 2
  },
  addButton: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    height: 42,
    borderRadius: 21,
    paddingHorizontal: spacing.lg,
    backgroundColor: colors.hero,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.heroBorder
  },
  onHero: {
    color: colors.onHero
  },
  onHeroMuted: {
    color: colors.onHeroMuted
  },
  pressed: {
    opacity: 0.7
  },
  scopeCard: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    borderRadius: radii.lg,
    backgroundColor: colors.hero,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.heroBorder,
    padding: 14,
    overflow: 'hidden'
  },
  scopeText: {
    flex: 1,
    minWidth: 0,
    gap: 2
  },
  scopeClose: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.heroControl
  },
  searchBox: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 44,
    backgroundColor: colors.surface,
    borderRadius: radii.full,
    paddingHorizontal: spacing.lg
  },
  searchInput: {
    flex: 1,
    color: colors.text,
    fontSize: 15
  },
  filters: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  filterChip: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 5,
    minHeight: 34,
    paddingHorizontal: spacing.md,
    borderRadius: radii.full,
    backgroundColor: colors.surface
  },
  filterChipActive: {
    backgroundColor: colors.text
  },
  filterText: {
    color: colors.muted
  },
  filterActiveText: {
    color: colors.background
  },
  listContent: {
    paddingBottom: tabBarClearance
  },
  dayHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    paddingHorizontal: spacing.xs,
    paddingTop: spacing.md,
    paddingBottom: spacing.sm
  },
  dayRule: {
    flex: 1,
    height: StyleSheet.hairlineWidth,
    backgroundColor: colors.borderLight
  },
  scopedLoading: {
    paddingVertical: spacing.xxl
  },
  footerSpinner: {
    paddingVertical: spacing.lg
  },
  txnCard: {
    backgroundColor: colors.surface,
    borderRadius: radii.lg,
    marginBottom: spacing.sm,
    overflow: 'hidden'
  },
  rowPressed: {
    opacity: 0.85
  },
  txnTop: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingHorizontal: 12,
    paddingVertical: 10
  },
  txnTile: {
    width: 40,
    height: 40,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  txnMain: {
    flex: 1,
    minWidth: 0,
    gap: 1
  },
  txnFoot: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 32,
    backgroundColor: colors.surfaceStrong,
    paddingHorizontal: 12,
    paddingVertical: 6
  },
  warnText: {
    color: colors.warning
  },
  approveButton: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    borderRadius: radii.full,
    backgroundColor: colors.successMuted,
    paddingHorizontal: spacing.sm + 2,
    paddingVertical: 3
  },
  tagRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.xs,
    marginTop: 3
  },
  tagChip: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 5,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: radii.full,
    paddingHorizontal: spacing.sm,
    paddingVertical: 1
  },
  tagDot: {
    width: 7,
    height: 7,
    borderRadius: 4
  }
});
