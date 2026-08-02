import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  FlatList,
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  TextInput,
  View
} from 'react-native';
import { useNavigation, useRoute, type RouteProp } from '@react-navigation/native';
import { Check, ChevronDown, ChevronUp, Plus, ReceiptText, Search, Sparkles, Trash2, X } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { EmptyState } from '../../../components/EmptyState';
import { PickerField, PickerOverlay, type PickerOption } from '../../../components/Picker';
import { apiClient } from '../../../utils/api';
import type { PaginationResponse } from '../../../utils/constants';
import { formatCurrency, formatShortDate, getSelectedMonthInHumanFormat } from '../../../utils/date';
import { colors, radii, spacing, tabBarClearance } from '../../../theme';
import type { AppTabParamList } from '../../../navigation/types';
import type { Tag } from '../../tags/types';
import type { Transaction, TransactionDTO, TransactionPredictionDetails } from '../types';
import { LocationSource, TransactionStatus } from '../types';
import { TransactionAttachments } from '../components/TransactionAttachments';
import { TransactionLocationRow, getCurrentCoords, type DraftLocation } from '../components/TransactionLocationRow';
import {
  createTransaction,
  deleteTransactionById,
  fetchAllTransactions,
  updateTransaction,
  updateTransactionStatus
} from '../store/transactionSlice';

type TxnDraft = {
  id?: string;
  accountId: string;
  payeeId: string;
  categoryId: string | null;
  date: string;
  amount: string;
  direction: 'outflow' | 'inflow';
  note: string;
  tagIds: string[];
  status?: TransactionStatus;
  location: DraftLocation;
};

type ActivePicker = 'account' | 'payee' | 'category' | null;

function createDraft(txn?: Transaction): TxnDraft {
  const inflow = txn?.inflow ?? 0;
  const outflow = txn?.outflow ?? 0;
  const magnitude = inflow > 0 ? inflow : outflow;
  return {
    id: txn?.id,
    accountId: txn?.accountId ?? '',
    payeeId: txn?.payeeId ?? '',
    categoryId: txn?.categoryId ?? null,
    date: txn?.date ?? new Date().toISOString().slice(0, 10),
    amount: txn ? String(Math.abs(magnitude || (txn.amount ?? 0))) : '',
    direction: inflow > 0 ? 'inflow' : 'outflow',
    note: txn?.note ?? '',
    tagIds: txn?.tagIds ?? [],
    status: txn?.status,
    location: {
      lat: txn?.locationLat ?? null,
      lng: txn?.locationLng ?? null,
      name: txn?.locationName ?? null,
      source: txn?.locationSource ?? null
    }
  };
}

function statusLabel(status?: TransactionStatus) {
  switch (status) {
    case TransactionStatus.APPROVED:
      return 'Approved';
    case TransactionStatus.REJECTED:
      return 'Rejected';
    case TransactionStatus.UNAPPROVED:
      return 'Needs approval';
    case TransactionStatus.MANUAL:
      return 'Manual entry';
    default:
      return null;
  }
}

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
          <AppText variant="caption" muted numberOfLines={1}>{tag.name}</AppText>
        </View>
      ))}
    </View>
  );
}

function formatConfidence(value?: number | null) {
  if (value === null || value === undefined) return '—';
  const percent = value <= 1 ? value * 100 : value;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function PredictionMetric({ label, value, confidence }: { label: string; value?: string | null; confidence?: number | null }) {
  return (
    <View style={styles.predictionMetric}>
      <AppText variant="label" tone="faint">{label}</AppText>
      <AppText variant="caption" weight="semibold" numberOfLines={2}>{value || '—'}</AppText>
      <AppText variant="caption" tone="primary" tabular>{formatConfidence(confidence)}</AppText>
    </View>
  );
}

function PredictionRow({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.predictionRow}>
      <AppText variant="caption" muted>{label}</AppText>
      <AppText variant="caption" weight="medium" numberOfLines={1} style={styles.predictionRowValue}>{value}</AppText>
    </View>
  );
}

function PredictionSection({ transactionId }: { transactionId: string }) {
  const payees = useAppSelector((state) => state.payees.allPayees);
  const categories = useAppSelector((state) => state.categories.allCategoryGroups.flatMap((group) => group.categories));
  const [isOpen, setIsOpen] = useState(false);
  // Latches true on the first expand and never clears. The fetch keys off this
  // rather than `isOpen` so that collapsing the panel -- or any state the effect
  // itself sets -- cannot re-run the effect and abort its own in-flight request.
  const [hasRequested, setHasRequested] = useState(false);
  const [details, setDetails] = useState<TransactionPredictionDetails | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!hasRequested) return;
    let ignore = false;
    setIsLoading(true);
    setError(null);
    apiClient
      .get<TransactionPredictionDetails>(`predictions/transactions/${transactionId}`)
      .then((data) => {
        if (!ignore) setDetails(data ?? {});
      })
      .catch((err: unknown) => {
        if (!ignore) {
          setDetails({});
          setError(err instanceof Error ? err.message : 'Failed to load prediction');
        }
      })
      .finally(() => {
        if (!ignore) setIsLoading(false);
      });
    return () => {
      ignore = true;
    };
  }, [hasRequested, transactionId]);

  const cipher = details?.cipherPrediction;
  const legacy = details?.prediction;

  return (
    <View style={styles.predictionSection}>
      <Pressable
        style={({ pressed }) => [styles.predictionHeader, pressed && styles.pressed]}
        onPress={() => {
          setIsOpen((open) => !open);
          setHasRequested(true);
        }}
      >
        <Sparkles size={15} color={colors.primary} />
        <AppText variant="caption" weight="semibold" style={styles.predictionTitle}>AI prediction</AppText>
        {isOpen ? <ChevronUp size={15} color={colors.faint} /> : <ChevronDown size={15} color={colors.faint} />}
      </Pressable>

      {isOpen ? (
        <View style={styles.predictionBody}>
          {isLoading ? (
            <View style={styles.predictionLoading}>
              <ActivityIndicator size="small" color={colors.primary} />
              <AppText variant="caption" muted>Loading prediction</AppText>
            </View>
          ) : error ? (
            <AppText variant="caption" tone="danger">{error}</AppText>
          ) : cipher ? (
            <>
              <View style={styles.predictionGrid}>
                <PredictionMetric label="Account" value={cipher.extractedAccount} confidence={cipher.accountConfidence} />
                <PredictionMetric
                  label="Payee"
                  value={payees.find((payee) => payee.id === cipher.predictedPayeeId)?.name || cipher.extractedPayee}
                  confidence={cipher.payeeConfidence}
                />
                <PredictionMetric
                  label="Category"
                  value={categories.find((category) => category.id === cipher.predictedCategoryId)?.name}
                  confidence={cipher.categoryConfidence}
                />
              </View>
              <PredictionRow label="Predicted amount" value={cipher.amount != null ? formatCurrency(cipher.amount) : '—'} />
              <PredictionRow label="Source" value={cipher.source || '—'} />
              <PredictionRow label="User corrected" value={cipher.hasUserCorrected ? 'Yes' : 'No'} />
              {cipher.llmReasoning ? (
                <View style={styles.reasoningBlock}>
                  <AppText variant="caption" muted>{cipher.llmReasoning}</AppText>
                </View>
              ) : null}
            </>
          ) : legacy ? (
            <>
              <View style={styles.predictionGrid}>
                <PredictionMetric label="Account" value={legacy.account} confidence={legacy.accountPrediction} />
                <PredictionMetric label="Payee" value={legacy.payee} confidence={legacy.payeePrediction} />
                <PredictionMetric label="Category" value={legacy.category} confidence={legacy.categoryPrediction} />
              </View>
              <PredictionRow label="Predicted amount" value={legacy.amount != null ? formatCurrency(legacy.amount) : '—'} />
              <PredictionRow label="User corrected" value={legacy.hasUserCorrected ? 'Yes' : 'No'} />
              {legacy.hasUserCorrected ? (
                <>
                  <PredictionRow label="Corrected account" value={legacy.userCorrectedAccount || '—'} />
                  <PredictionRow label="Corrected payee" value={legacy.userCorrectedPayee || '—'} />
                  <PredictionRow label="Corrected category" value={legacy.userCorrectedCategory || '—'} />
                </>
              ) : null}
            </>
          ) : (
            <AppText variant="caption" muted>No prediction found for this transaction.</AppText>
          )}
        </View>
      ) : null}
    </View>
  );
}

function TransactionEditor({
  draft,
  setDraft,
  onClose,
  onSave,
  onDeleted
}: {
  draft: TxnDraft | null;
  setDraft: (draft: TxnDraft) => void;
  onClose: () => void;
  onSave: () => void;
  onDeleted: () => void;
}) {
  const dispatch = useAppDispatch();
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const payees = useAppSelector((state) => state.payees.allPayees);
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  const tags = useAppSelector((state) => state.tags.tags);
  const [activePicker, setActivePicker] = useState<ActivePicker>(null);

  const accountOptions: PickerOption[] = useMemo(
    () => accounts.filter((account) => account.id).map((account) => ({ id: account.id as string, label: account.name, sublabel: account.type })),
    [accounts]
  );
  const payeeOptions: PickerOption[] = useMemo(
    () => payees.filter((payee) => payee.id).map((payee) => ({ id: payee.id as string, label: payee.name })),
    [payees]
  );
  const categoryOptions: PickerOption[] = useMemo(
    () =>
      groups.flatMap((group) =>
        group.categories
          .filter((category) => category.id)
          .map((category) => ({ id: category.id as string, label: category.name, sublabel: group.name }))
      ),
    [groups]
  );

  if (!draft) return null;

  const label = statusLabel(draft.status);
  const accountName = accountOptions.find((option) => option.id === draft.accountId)?.label;
  const payeeName = payeeOptions.find((option) => option.id === draft.payeeId)?.label;
  const categoryName = categoryOptions.find((option) => option.id === draft.categoryId)?.label;

  const setStatus = (status: TransactionStatus) => {
    if (!draft.id) return;
    dispatch(updateTransactionStatus({ id: draft.id, status }));
    onClose();
  };

  const confirmDelete = () => {
    if (!draft.id) return;
    Alert.alert('Delete transaction?', 'This removes the transaction from its account and budget.', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: async () => {
          await dispatch(deleteTransactionById(draft.id as string)).unwrap();
          onDeleted();
        }
      }
    ]);
  };

  const toggleTag = (tagId?: string) => {
    if (!tagId) return;
    const tagIds = draft.tagIds.includes(tagId) ? draft.tagIds.filter((id) => id !== tagId) : [...draft.tagIds, tagId];
    setDraft({ ...draft, tagIds });
  };

  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose}>
      <KeyboardAvoidingView style={styles.modalBackdrop} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
        <View style={styles.sheet}>
          <View style={styles.sheetHandle} />
          <View style={styles.rowBetween}>
            <AppText variant="title" style={styles.sheetTitle}>{draft.id ? 'Edit transaction' : 'New transaction'}</AppText>
            <Pressable style={({ pressed }) => [styles.closeButton, pressed && styles.pressed]} onPress={onClose}>
              <X size={18} color={colors.muted} />
            </Pressable>
          </View>

          <ScrollView
            style={styles.sheetScrollView}
            contentContainerStyle={styles.sheetScroll}
            showsVerticalScrollIndicator={false}
            keyboardShouldPersistTaps="handled"
          >
            <AppText variant="label" tone="faint">Amount</AppText>
            <View style={styles.amountRow}>
              <View style={styles.directionToggle}>
                <Pressable
                  style={[styles.directionOption, draft.direction === 'outflow' && styles.directionOutflow]}
                  onPress={() => setDraft({ ...draft, direction: 'outflow' })}
                >
                  <AppText variant="caption" weight="semibold" style={draft.direction === 'outflow' ? styles.directionOutflowText : styles.directionText}>
                    Outflow
                  </AppText>
                </Pressable>
                <Pressable
                  style={[styles.directionOption, draft.direction === 'inflow' && styles.directionInflow]}
                  onPress={() => setDraft({ ...draft, direction: 'inflow' })}
                >
                  <AppText variant="caption" weight="semibold" style={draft.direction === 'inflow' ? styles.directionInflowText : styles.directionText}>
                    Inflow
                  </AppText>
                </Pressable>
              </View>
              <TextInput
                value={draft.amount}
                onChangeText={(amount) => setDraft({ ...draft, amount })}
                style={[styles.input, styles.amountInput]}
                keyboardType="numeric"
                placeholder="0"
                placeholderTextColor={colors.faint}
              />
            </View>

            <AppText variant="label" tone="faint">Date</AppText>
            <TextInput
              value={draft.date}
              onChangeText={(date) => setDraft({ ...draft, date })}
              style={styles.input}
              placeholder="YYYY-MM-DD"
              placeholderTextColor={colors.faint}
            />

            <PickerField label="Account" value={accountName} onPress={() => setActivePicker('account')} />
            <PickerField label="Payee" value={payeeName} onPress={() => setActivePicker('payee')} />
            <PickerField label="Category" value={categoryName} placeholder="Uncategorized" onPress={() => setActivePicker('category')} />

            {tags.length ? (
              <>
                <AppText variant="label" tone="faint">Tags</AppText>
                <View style={styles.tagPickRow}>
                  {tags.map((tag) => {
                    const selected = Boolean(tag.id && draft.tagIds.includes(tag.id));
                    return (
                      <Pressable
                        key={tag.id ?? tag.name}
                        style={[styles.choice, styles.tagChoice, selected && styles.choiceSelected]}
                        onPress={() => toggleTag(tag.id)}
                      >
                        <View style={[styles.tagDot, { backgroundColor: tag.color || colors.primary }]} />
                        <AppText variant="caption" weight="medium" style={selected ? styles.choiceSelectedText : styles.choiceText}>
                          {tag.name}
                        </AppText>
                      </Pressable>
                    );
                  })}
                </View>
              </>
            ) : null}

            <AppText variant="label" tone="faint">Note</AppText>
            <TextInput
              value={draft.note}
              onChangeText={(note) => setDraft({ ...draft, note })}
              style={styles.input}
              placeholder="Note"
              placeholderTextColor={colors.faint}
            />

            <TransactionLocationRow
              transactionId={draft.id}
              location={draft.location}
              onDraftChange={(location) => setDraft({ ...draft, location })}
            />

            {draft.id ? <TransactionAttachments transactionId={draft.id} /> : null}

            {draft.id ? <PredictionSection key={draft.id} transactionId={draft.id} /> : null}

            {draft.id && label ? (
              <View style={styles.statusRow}>
                <AppText variant="caption" muted>Status: {label}</AppText>
                {draft.status === TransactionStatus.UNAPPROVED ? (
                  <View style={styles.statusActions}>
                    <Button size="sm" variant="secondary" onPress={() => setStatus(TransactionStatus.REJECTED)}>
                      Reject
                    </Button>
                    <Button size="sm" onPress={() => setStatus(TransactionStatus.APPROVED)}>
                      Approve
                    </Button>
                  </View>
                ) : null}
              </View>
            ) : null}
          </ScrollView>

          <View style={styles.actionRow}>
            {draft.id ? (
              <Pressable
                accessibilityRole="button"
                accessibilityLabel="Delete transaction"
                style={({ pressed }) => [styles.deleteButton, pressed && styles.pressed]}
                onPress={confirmDelete}
              >
                <Trash2 size={18} color={colors.danger} />
              </Pressable>
            ) : null}
            <Button size="sm" style={styles.saveButton} onPress={onSave}>
              Save
            </Button>
          </View>
        </View>

        {activePicker === 'account' ? (
          <PickerOverlay
            title="Account"
            options={accountOptions}
            selectedId={draft.accountId}
            searchPlaceholder="Search accounts"
            onSelect={(id) => {
              if (id) setDraft({ ...draft, accountId: id });
              setActivePicker(null);
            }}
            onClose={() => setActivePicker(null)}
          />
        ) : null}
        {activePicker === 'payee' ? (
          <PickerOverlay
            title="Payee"
            options={payeeOptions}
            selectedId={draft.payeeId}
            searchPlaceholder="Search payees"
            onSelect={(id) => {
              if (id) setDraft({ ...draft, payeeId: id });
              setActivePicker(null);
            }}
            onClose={() => setActivePicker(null)}
          />
        ) : null}
        {activePicker === 'category' ? (
          <PickerOverlay
            title="Category"
            options={categoryOptions}
            selectedId={draft.categoryId}
            searchPlaceholder="Search categories"
            clearLabel="Uncategorized"
            onSelect={(id) => {
              setDraft({ ...draft, categoryId: id });
              setActivePicker(null);
            }}
            onClose={() => setActivePicker(null)}
          />
        ) : null}
      </KeyboardAvoidingView>
    </Modal>
  );
}

export function TransactionsScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation();
  const route = useRoute<RouteProp<AppTabParamList, 'Transactions'>>();
  const selectedBudget = useAppSelector((state) => state.budgets.selectedBudget);
  const { transactions, nextCursor, loadingMore, loading } = useAppSelector((state) => state.transactions);
  const tags = useAppSelector((state) => state.tags.tags);
  const [search, setSearch] = useState('');
  const [draft, setDraft] = useState<TxnDraft | null>(null);
  const [scoped, setScoped] = useState<Transaction[] | null>(null);
  const [isScopedLoading, setIsScopedLoading] = useState(false);

  const categoryId = route.params?.categoryId;
  const categoryName = route.params?.categoryName;
  const month = route.params?.month;

  const loadScoped = useCallback(() => {
    if (!categoryId) {
      setScoped(null);
      return;
    }
    const params = new URLSearchParams();
    params.set('categoryId[]', categoryId);
    if (month) {
      const { startDate, endDate } = monthRange(month);
      params.set('startDate', startDate);
      params.set('endDate', endDate);
    }
    params.set('limit', '100');
    setIsScopedLoading(true);
    apiClient
      .get<PaginationResponse<Transaction[]>>(`transactions/normalized?${params.toString()}`)
      .then((response) => setScoped(response.data ?? []))
      .catch(() => setScoped([]))
      .finally(() => setIsScopedLoading(false));
  }, [categoryId, month]);

  useEffect(loadScoped, [loadScoped]);

  const clearScope = () => {
    setScoped(null);
    navigation.setParams({ categoryId: undefined, categoryName: undefined, month: undefined } as never);
  };

  const source = categoryId ? scoped ?? [] : transactions;

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return source;
    return source.filter((txn) =>
      [txn.payeeName, txn.accountName, txn.categoryName ?? '', txn.note ?? ''].some((value) => value.toLowerCase().includes(q))
    );
  }, [search, source]);

  const refreshLists = () => {
    dispatch(fetchAllTransactions());
    if (categoryId) loadScoped();
  };

  const save = async () => {
    if (!draft || !selectedBudget?.id) return;
    const magnitude = Number(draft.amount.replace(/,/g, ''));
    if (!draft.accountId || !draft.payeeId || !Number.isFinite(magnitude)) return;
    const amount = draft.direction === 'inflow' ? Math.abs(magnitude) : -Math.abs(magnitude);
    const payload: TransactionDTO = {
      id: draft.id,
      budgetId: selectedBudget.id,
      accountId: draft.accountId,
      payeeId: draft.payeeId,
      categoryId: draft.categoryId,
      date: draft.date,
      amount,
      note: draft.note,
      status: draft.status,
      tagIds: draft.tagIds,
      // pass location through so a plain edit doesn't wipe an existing pin
      locationLat: draft.location.lat,
      locationLng: draft.location.lng,
      locationName: draft.location.name,
      locationSource: draft.location.lat != null ? (draft.location.source ?? LocationSource.MANUAL) : null
    };
    if (draft.id) await dispatch(updateTransaction(payload)).unwrap();
    else await dispatch(createTransaction(payload)).unwrap();
    setDraft(null);
    refreshLists();
  };

  // new transactions auto-attach the phone's location (removable before save)
  const openNewDraft = () => {
    setDraft(createDraft());
    void getCurrentCoords().then((coords) => {
      if (!coords) return;
      setDraft((current) =>
        !current || current.id || current.location.lat != null
          ? current
          : { ...current, location: { lat: coords.lat, lng: coords.lng, name: null, source: LocationSource.AUTO } }
      );
    });
  };

  return (
    <Screen scroll={false} style={styles.screen}>
      <View style={styles.header}>
        <View style={styles.headerText}>
          <AppText variant="title">Transactions</AppText>
          <AppText variant="caption" muted>
            {categoryId ? `${filtered.length} in this category` : `${transactions.length} loaded`}
          </AppText>
        </View>
        <Pressable style={({ pressed }) => [styles.addButton, pressed && styles.pressed]} onPress={openNewDraft}>
          <Plus size={22} color={colors.onPrimary} />
        </Pressable>
      </View>

      {categoryId ? (
        <Pressable style={({ pressed }) => [styles.scopeChip, pressed && styles.pressed]} onPress={clearScope}>
          <AppText variant="caption" weight="semibold" tone="primary" numberOfLines={1} style={styles.scopeText}>
            {categoryName ?? 'Category'}
            {month ? ` · ${getSelectedMonthInHumanFormat(month)}` : ''}
          </AppText>
          <X size={14} color={colors.primary} />
        </Pressable>
      ) : null}

      <View style={styles.searchBox}>
        <Search size={17} color={colors.faint} />
        <TextInput
          value={search}
          onChangeText={setSearch}
          placeholder="Search transactions"
          placeholderTextColor={colors.faint}
          style={styles.searchInput}
        />
      </View>

      <FlatList
        data={filtered}
        keyExtractor={(item) => item.id ?? `${item.date}-${item.payeeName}-${item.amount}`}
        contentContainerStyle={styles.listContent}
        showsVerticalScrollIndicator={false}
        refreshControl={
          <RefreshControl
            // The list is served either from the store or from the scoped
            // category fetch, so the spinner has to track whichever is live.
            refreshing={categoryId ? isScopedLoading : loading === 'pending'}
            onRefresh={refreshLists}
            tintColor={colors.primary}
          />
        }
        ListEmptyComponent={
          isScopedLoading ? (
            <View style={styles.scopedLoading}>
              <ActivityIndicator color={colors.primary} />
            </View>
          ) : (
            <EmptyState
              icon={<ReceiptText size={26} color={colors.primary} />}
              title={categoryId ? 'No transactions' : 'No transactions yet'}
              body={
                categoryId
                  ? 'Nothing was spent in this category for the selected month.'
                  : 'Transactions from your accounts and Gmail imports will show up here.'
              }
            />
          )
        }
        onEndReached={() => {
          if (!categoryId && nextCursor && loadingMore !== 'pending') dispatch(fetchAllTransactions({ cursor: nextCursor }));
        }}
        renderItem={({ item }) => {
          const amount = (item.inflow ?? 0) || -(item.outflow ?? 0);
          const unapproved = item.status === TransactionStatus.UNAPPROVED;
          return (
            <Pressable
              android_ripple={{ color: colors.surfaceTertiary }}
              style={({ pressed }) => [styles.txnRow, pressed && styles.rowPressed]}
              onPress={() => setDraft(createDraft(item))}
            >
              <View style={styles.txnMain}>
                <AppText weight="medium" numberOfLines={1}>{item.payeeName || 'Unknown payee'}</AppText>
                <AppText variant="caption" muted numberOfLines={1}>
                  {formatShortDate(item.date)} · {item.categoryName ?? 'Uncategorized'} · {item.accountName}
                </AppText>
                {item.note ? <AppText variant="caption" tone="faint" numberOfLines={1}>{item.note}</AppText> : null}
                <TagChips tagIds={item.tagIds ?? []} tags={tags} />
              </View>
              <View style={styles.amountBlock}>
                <AppText weight="semibold" tone={amount > 0 ? 'success' : 'default'} tabular>
                  {formatCurrency(amount, { signed: true })}
                </AppText>
                {unapproved ? (
                  <Pressable
                    style={({ pressed }) => [styles.approveButton, pressed && styles.pressed]}
                    onPress={() => item.id && dispatch(updateTransactionStatus({ id: item.id, status: TransactionStatus.APPROVED }))}
                  >
                    <Check size={13} color={colors.success} />
                    <AppText variant="caption" weight="semibold" tone="success">Approve</AppText>
                  </Pressable>
                ) : null}
              </View>
            </Pressable>
          );
        }}
      />

      <TransactionEditor
        draft={draft}
        setDraft={setDraft}
        onClose={() => setDraft(null)}
        onSave={() => void save()}
        onDeleted={() => {
          setDraft(null);
          refreshLists();
        }}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
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
    width: 44,
    height: 44,
    borderRadius: 22,
    backgroundColor: colors.primary,
    alignItems: 'center',
    justifyContent: 'center'
  },
  scopeChip: {
    flexDirection: 'row',
    alignItems: 'center',
    alignSelf: 'flex-start',
    gap: spacing.sm,
    borderRadius: radii.full,
    backgroundColor: colors.primaryMuted,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm - 2
  },
  scopeText: {
    flexShrink: 1
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
  scopedLoading: {
    paddingVertical: spacing.xxl
  },
  txnRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    backgroundColor: colors.surface,
    borderRadius: radii.md,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    overflow: 'hidden'
  },
  rowPressed: {
    backgroundColor: colors.surfaceStrong
  },
  txnMain: {
    flex: 1,
    gap: 2
  },
  amountBlock: {
    alignItems: 'flex-end',
    gap: spacing.sm
  },
  approveButton: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    borderRadius: radii.full,
    backgroundColor: colors.successMuted,
    paddingHorizontal: spacing.sm + 2,
    paddingVertical: 4
  },
  tagRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.xs,
    marginTop: 2
  },
  tagChip: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 5,
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.full,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2
  },
  tagDot: {
    width: 7,
    height: 7,
    borderRadius: 4
  },
  pressed: {
    opacity: 0.7
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
    maxHeight: '90%',
    backgroundColor: colors.surface,
    borderTopLeftRadius: radii.xl,
    borderTopRightRadius: radii.xl,
    paddingHorizontal: spacing.xl,
    paddingTop: spacing.md,
    paddingBottom: spacing.lg,
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
  sheetTitle: {
    fontSize: 20,
    lineHeight: 26
  },
  // Without an explicit shrink the ScrollView sizes to its content and the
  // sheet's maxHeight clips it instead of scrolling.
  sheetScrollView: {
    flexShrink: 1
  },
  sheetScroll: {
    gap: spacing.md,
    paddingTop: spacing.sm,
    paddingBottom: spacing.md
  },
  closeButton: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  input: {
    minHeight: 48,
    borderRadius: radii.md,
    paddingHorizontal: spacing.lg,
    color: colors.text,
    backgroundColor: colors.surfaceStrong,
    fontSize: 15
  },
  amountRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  amountInput: {
    flex: 1,
    textAlign: 'right',
    fontVariant: ['tabular-nums']
  },
  directionToggle: {
    flexDirection: 'row',
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.full,
    padding: 3
  },
  directionOption: {
    minHeight: 36,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: radii.full
  },
  directionText: {
    color: colors.muted
  },
  directionOutflow: {
    backgroundColor: colors.dangerMuted
  },
  directionOutflowText: {
    color: colors.danger
  },
  directionInflow: {
    backgroundColor: colors.successMuted
  },
  directionInflowText: {
    color: colors.success
  },
  choice: {
    minHeight: 36,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong
  },
  tagChoice: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6
  },
  tagPickRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sm
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
  predictionSection: {
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.md,
    overflow: 'hidden'
  },
  predictionHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 44,
    paddingHorizontal: spacing.lg
  },
  predictionTitle: {
    flex: 1
  },
  predictionBody: {
    gap: spacing.sm,
    paddingHorizontal: spacing.lg,
    paddingBottom: spacing.lg
  },
  predictionLoading: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 32
  },
  predictionGrid: {
    flexDirection: 'row',
    gap: spacing.sm,
    marginBottom: spacing.xs
  },
  predictionMetric: {
    flex: 1,
    gap: 3,
    backgroundColor: colors.surface,
    borderRadius: radii.sm,
    padding: spacing.md
  },
  predictionRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  predictionRowValue: {
    flexShrink: 1,
    textAlign: 'right'
  },
  reasoningBlock: {
    backgroundColor: colors.surface,
    borderRadius: radii.sm,
    padding: spacing.md,
    marginTop: spacing.xs
  },
  statusRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  statusActions: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  actionRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  saveButton: {
    flex: 1
  },
  deleteButton: {
    width: 44,
    height: 44,
    borderRadius: 22,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.dangerMuted
  }
});
