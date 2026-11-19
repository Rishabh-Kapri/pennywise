import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Alert, Keyboard, Modal, Pressable, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import {
  ArrowLeftRight,
  CalendarDays,
  Check,
  ChevronRight,
  FolderOpen,
  NotebookPen,
  Store,
  Trash2,
  WalletCards
} from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AmountKeypad, ExpressionPreview, pressKey } from '../../../components/AmountKeypad';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { DatePickerOverlay } from '../../../components/DatePicker';
import { Hatch } from '../../../components/Hatch';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { PickerOverlay, type PickerOption } from '../../../components/Picker';
import { SheetOverlay } from '../../../components/Sheet';
import { colors, radii, spacing } from '../../../theme';
import { formatCurrency, formatRelativeDay, todayKey } from '../../../utils/date';
import { evaluateAmount } from '../../../utils/expression';
import { createPayee } from '../../payees/store/payeeSlice';
import type { Transaction, TransactionDTO } from '../types';
import { LocationSource, TransactionStatus } from '../types';
import {
  createTransaction,
  deleteTransactionById,
  updateTransaction,
  updateTransactionStatus,
  isVersionConflict
} from '../store/transactionSlice';
import { TransactionAttachments } from './TransactionAttachments';
import { TransactionLocationRow, getCurrentCoords, type DraftLocation } from './TransactionLocationRow';
import { PredictionSection } from './TransactionPrediction';

export type TxnDraft = {
  id?: string;
  accountId: string;
  payeeId: string;
  categoryId: string | null;
  date: string;
  /** what was typed, possibly a sum like "450+120" */
  amount: string;
  direction: 'outflow' | 'inflow';
  note: string;
  tagIds: string[];
  status?: TransactionStatus;
  location: DraftLocation;
  /** The version the draft was opened at, sent as If-Match when saving. */
  version?: number;
};

/** A failed write in words; a version conflict says what happened and what to do. */
function errorLine(err: unknown, fallback: string): string {
  if (isVersionConflict(err)) {
    return 'This transaction changed on another device or in the pipeline. Close it and open it again to see the latest, then make your change.';
  }
  if (err && typeof err === 'object' && 'message' in err && typeof err.message === 'string' && err.message) {
    return err.message;
  }
  return fallback;
}

export function createDraft(txn?: Transaction): TxnDraft {
  const inflow = txn?.inflow ?? 0;
  const outflow = txn?.outflow ?? 0;
  const magnitude = inflow > 0 ? inflow : outflow;
  return {
    id: txn?.id,
    accountId: txn?.accountId ?? '',
    payeeId: txn?.payeeId ?? '',
    categoryId: txn?.categoryId ?? null,
    date: txn?.date?.slice(0, 10) ?? todayKey(),
    amount: txn ? String(Math.abs(magnitude || (txn.amount ?? 0))) : '',
    direction: inflow > 0 ? 'inflow' : 'outflow',
    note: txn?.note ?? '',
    tagIds: txn?.tagIds ?? [],
    status: txn?.status,
    version: txn?.version,
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
      return 'Needs review';
    case TransactionStatus.MANUAL:
      return 'Manual entry';
    default:
      return null;
  }
}

type Overlay = 'account' | 'payee' | 'category' | 'date' | null;

/** One tappable line of the form: icon, label, current value. */
function FieldRow({
  icon,
  label,
  value,
  placeholder,
  onPress,
  warn
}: {
  icon: ReactNode;
  label: string;
  value?: string | null;
  placeholder: string;
  onPress: () => void;
  warn?: boolean;
}) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`${label}: ${value || placeholder}`}
      style={({ pressed }) => [styles.fieldRow, pressed && styles.fieldRowPressed]}
      onPress={onPress}
    >
      <View style={styles.fieldIcon}>{icon}</View>
      <View style={styles.fieldText}>
        <AppText variant="caption" tone="faint">
          {label}
        </AppText>
        <AppText numberOfLines={1} weight={value ? 'medium' : 'regular'} style={!value && (warn ? styles.warnText : styles.placeholder)}>
          {value || placeholder}
        </AppText>
      </View>
      <ChevronRight size={18} color={colors.faint} />
    </Pressable>
  );
}

/**
 * Full-height sheet for adding or editing a transaction. Render it only while
 * a draft is open: every open starts with no picker showing and the keypad
 * in its default state.
 */
export function TransactionEditor({
  initialDraft,
  onClose,
  onSaved
}: {
  initialDraft: TxnDraft;
  onClose: () => void;
  /** after a save or a delete, so the caller can refresh its lists */
  onSaved: () => void;
}) {
  const dispatch = useAppDispatch();
  const selectedBudget = useAppSelector((state) => state.budgets.selectedBudget);
  const selectedMonth = useAppSelector((state) => state.budgets.selectedMonth);
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const payees = useAppSelector((state) => state.payees.allPayees);
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  const tags = useAppSelector((state) => state.tags.tags);

  const [draft, setDraft] = useState(initialDraft);
  const [overlay, setOverlay] = useState<Overlay>(null);
  // a new transaction starts on the amount; an edit starts collapsed
  const [keypadOpen, setKeypadOpen] = useState(!initialDraft.id);
  const [freshAmount, setFreshAmount] = useState(Boolean(initialDraft.id));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const update = (patch: Partial<TxnDraft>) => setDraft((current) => ({ ...current, ...patch }));

  // a new transaction picks up the phone's location (removable before save)
  useEffect(() => {
    if (initialDraft.id || initialDraft.location.lat != null) return;
    let cancelled = false;
    void getCurrentCoords().then((coords) => {
      if (!coords || cancelled) return;
      setDraft((current) =>
        current.location.lat != null
          ? current
          : { ...current, location: { lat: coords.lat, lng: coords.lng, name: null, source: LocationSource.AUTO } }
      );
    });
    return () => {
      cancelled = true;
    };
  }, [initialDraft]);

  const accountOptions: PickerOption[] = useMemo(
    () =>
      accounts
        .filter((account) => account.id && !account.closed && !account.deleted)
        .map((account) => ({
          id: account.id as string,
          label: account.name,
          sublabel: formatCurrency(account.balance ?? 0)
        })),
    [accounts]
  );

  const payeeOptions: PickerOption[] = useMemo(() => {
    const live = payees.filter((payee) => payee.id && !payee.deleted);
    const byName = (a: { name: string }, b: { name: string }) => a.name.localeCompare(b.name);
    return [
      ...live
        .filter((payee) => !payee.transferAccountId)
        .sort(byName)
        .map((payee) => ({ id: payee.id as string, label: payee.name, group: 'Payees' })),
      ...live
        .filter((payee) => payee.transferAccountId)
        .sort(byName)
        .map((payee) => ({ id: payee.id as string, label: payee.name, group: 'Transfers' }))
    ];
  }, [payees]);

  // Inflow (Ready to Assign) first, like the web picker, then the budget's
  // groups with each category's available balance for the month.
  const categoryOptions: PickerOption[] = useMemo(() => {
    const month = draft.date.slice(0, 7) || selectedMonth;
    const ordered = [...groups.filter((group) => group.isSystem), ...groups.filter((group) => !group.isSystem)];
    return ordered
      .filter((group) => group.name !== 'Hidden')
      .flatMap((group) =>
        group.categories
          .filter((category) => category.id && !category.hidden && !category.deleted)
          .map((category) => {
            const available = category.balance?.[month];
            return {
              id: category.id as string,
              label: category.name,
              sublabel: group.isSystem || available === undefined ? undefined : `${formatCurrency(available)} available`,
              group: group.isSystem ? 'Inflow' : group.name
            };
          })
      );
  }, [groups, draft.date, selectedMonth]);

  const accountName = accountOptions.find((option) => option.id === draft.accountId)?.label
    ?? accounts.find((account) => account.id === draft.accountId)?.name;
  const payeeName = payeeOptions.find((option) => option.id === draft.payeeId)?.label;
  const categoryName = categoryOptions.find((option) => option.id === draft.categoryId)?.label;
  const isTransfer = Boolean(payees.find((payee) => payee.id === draft.payeeId)?.transferAccountId);

  const amountValue = evaluateAmount(draft.amount);
  const missing = [
    amountValue === null || amountValue === 0 ? 'an amount' : null,
    draft.payeeId ? null : 'a payee',
    draft.accountId ? null : 'an account'
  ].filter(Boolean);
  const label = statusLabel(draft.status);

  const onKey = (key: string) => {
    setDraft((current) => {
      // typing straight after opening an existing amount replaces it
      const base = freshAmount && /^[\d.]$/.test(key) ? '' : current.amount;
      return { ...current, amount: pressKey(base, key) };
    });
    setFreshAmount(false);
  };

  const openOverlay = (next: Overlay) => {
    Keyboard.dismiss();
    setKeypadOpen(false);
    setOverlay(next);
  };

  const handleBack = () => {
    if (overlay) setOverlay(null);
    else onClose();
  };

  const save = async () => {
    if (!selectedBudget?.id || missing.length || amountValue === null || saving) return;
    const magnitude = Math.abs(amountValue);
    const payload: TransactionDTO = {
      id: draft.id,
      budgetId: selectedBudget.id,
      accountId: draft.accountId,
      payeeId: draft.payeeId,
      categoryId: draft.categoryId,
      date: draft.date,
      amount: draft.direction === 'inflow' ? magnitude : -magnitude,
      note: draft.note,
      // saving a transaction the AI created is the review, as on the web
      status: draft.status === TransactionStatus.UNAPPROVED ? TransactionStatus.APPROVED : draft.status,
      tagIds: draft.tagIds,
      // pass location through so a plain edit doesn't wipe an existing pin
      locationLat: draft.location.lat,
      locationLng: draft.location.lng,
      locationName: draft.location.name,
      locationSource: draft.location.lat != null ? (draft.location.source ?? LocationSource.MANUAL) : null,
      version: draft.version
    };
    setSaving(true);
    setError(null);
    try {
      if (draft.id) await dispatch(updateTransaction(payload)).unwrap();
      else await dispatch(createTransaction(payload)).unwrap();
      onSaved();
    } catch (err) {
      setError(errorLine(err, "Couldn't save the transaction"));
    } finally {
      setSaving(false);
    }
  };

  const setStatus = (status: TransactionStatus) => {
    if (!draft.id) return;
    dispatch(updateTransactionStatus({ id: draft.id, status, version: draft.version }));
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
          try {
            await dispatch(deleteTransactionById({ id: draft.id as string, version: draft.version })).unwrap();
            onSaved();
          } catch (err) {
            Alert.alert("Couldn't delete the transaction", errorLine(err, 'Please try again.'));
          }
        }
      }
    ]);
  };

  const createAndPickPayee = async (name: string) => {
    try {
      const created = await dispatch(createPayee({ name })).unwrap();
      if (created.id) update({ payeeId: created.id });
      setOverlay(null);
    } catch (err) {
      Alert.alert("Couldn't add payee", err instanceof Error ? err.message : 'Please try again.');
    }
  };

  const toggleTag = (tagId?: string) => {
    if (!tagId) return;
    setDraft((current) => ({
      ...current,
      tagIds: current.tagIds.includes(tagId) ? current.tagIds.filter((id) => id !== tagId) : [...current.tagIds, tagId]
    }));
  };

  const inflow = draft.direction === 'inflow';

  return (
    <Modal
      visible
      transparent
      statusBarTranslucent
      navigationBarTranslucent
      animationType="slide"
      onRequestClose={handleBack}
    >
      <SheetOverlay
        title={draft.id ? 'Transaction details' : 'New transaction'}
        onClose={onClose}
        size="tall"
        footer={
          <View style={styles.footer}>
            {error ? (
              <AppText variant="caption" tone="danger">
                {error}
              </AppText>
            ) : missing.length && !keypadOpen ? (
              <AppText variant="caption" muted>
                Add {missing.join(', ')} to save
              </AppText>
            ) : null}
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
              {draft.id && draft.status === TransactionStatus.UNAPPROVED ? (
                <Button variant="secondary" style={styles.footerButton} onPress={() => setStatus(TransactionStatus.REJECTED)}>
                  Reject
                </Button>
              ) : null}
              <Button
                style={styles.footerButton}
                disabled={Boolean(missing.length) || saving}
                onPress={() => void save()}
              >
                {saving
                  ? 'Saving…'
                  : draft.id && draft.status === TransactionStatus.UNAPPROVED
                    ? 'Approve & save'
                    : 'Save'}
              </Button>
            </View>
          </View>
        }
      >
        <ScrollView
          contentContainerStyle={styles.scroll}
          showsVerticalScrollIndicator={false}
          keyboardShouldPersistTaps="handled"
        >
          {draft.id ? (
            <View style={styles.summary}>
              <View style={styles.summaryTop}>
                <View style={styles.avatarTile}>
                  {isTransfer ? <ArrowLeftRight size={20} color={colors.muted} /> : <InitialAvatar name={payeeName ?? '?'} size={36} />}
                </View>
                {label ? (
                  <View style={[styles.statusBadge, draft.status === TransactionStatus.UNAPPROVED && styles.statusBadgeReview]}>
                    <AppText
                      variant="caption"
                      weight="semibold"
                      tone={draft.status === TransactionStatus.UNAPPROVED ? 'primary' : draft.status === TransactionStatus.REJECTED ? 'danger' : 'success'}
                    >
                      {label}
                    </AppText>
                  </View>
                ) : null}
              </View>
              <AppText variant="title" numberOfLines={2}>
                {payeeName ?? 'Unknown payee'}
              </AppText>
              <AppText variant="caption" muted numberOfLines={1}>
                {[isTransfer && !draft.categoryId ? 'Transfer' : categoryName ?? 'Uncategorized', accountName, formatRelativeDay(draft.date)]
                  .filter(Boolean)
                  .join(' · ')}
              </AppText>
            </View>
          ) : null}

          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Amount"
            onPress={() => {
              Keyboard.dismiss();
              setKeypadOpen((open) => !open);
            }}
            style={[styles.amountCard, keypadOpen && styles.amountCardActive]}
          >
            <Hatch color={colors.onHero} gap={9} opacity={0.05} />
            <View style={styles.directionToggle}>
              {(['outflow', 'inflow'] as const).map((direction) => {
                const active = draft.direction === direction;
                return (
                  <Pressable
                    key={direction}
                    accessibilityRole="button"
                    accessibilityState={{ selected: active }}
                    style={[styles.directionOption, active && styles.directionActive]}
                    onPress={() => update({ direction })}
                  >
                    <AppText
                      variant="caption"
                      weight="semibold"
                      style={active ? (direction === 'outflow' ? styles.outflowText : styles.inflowText) : styles.directionText}
                    >
                      {direction === 'outflow' ? 'Outflow' : 'Inflow'}
                    </AppText>
                  </Pressable>
                );
              })}
            </View>
            <View style={styles.amountRow}>
              <AppText variant="display" tabular style={[styles.amountSign, inflow ? styles.inflowText : styles.outflowText]}>
                {inflow ? '+' : '−'}₹
              </AppText>
              <View style={styles.amountValue}>
                <ExpressionPreview expression={draft.amount} onDark />
              </View>
            </View>
            <AppText variant="caption" style={styles.amountHint}>
              {keypadOpen ? 'Sums work too, like 450+120' : 'Tap to change the amount'}
            </AppText>
          </Pressable>

          {keypadOpen ? (
            <AmountKeypad
              onKey={onKey}
              onDone={() => {
                const value = evaluateAmount(draft.amount);
                if (value !== null) update({ amount: String(Math.abs(value)) });
                setKeypadOpen(false);
              }}
            />
          ) : null}

          <View style={styles.fieldGroup}>
            <FieldRow
              icon={<Store size={18} color={colors.text} />}
              label="Payee"
              value={payeeName}
              placeholder="Choose payee"
              warn
              onPress={() => openOverlay('payee')}
            />
            <View style={styles.divider} />
            <FieldRow
              icon={<FolderOpen size={18} color={colors.text} />}
              label="Category"
              value={isTransfer && !draft.categoryId ? 'Transfer' : categoryName}
              placeholder="Uncategorized"
              onPress={() => openOverlay('category')}
            />
            <View style={styles.divider} />
            <FieldRow
              icon={<WalletCards size={18} color={colors.text} />}
              label="Account"
              value={accountName}
              placeholder="Choose account"
              warn
              onPress={() => openOverlay('account')}
            />
            <View style={styles.divider} />
            <FieldRow
              icon={<CalendarDays size={18} color={colors.text} />}
              label="Date"
              value={formatRelativeDay(draft.date)}
              placeholder="Pick a date"
              onPress={() => openOverlay('date')}
            />
            <View style={styles.divider} />
            <View style={styles.fieldRow}>
              <View style={styles.fieldIcon}>
                <NotebookPen size={18} color={colors.text} />
              </View>
              <TextInput
                value={draft.note}
                onChangeText={(note) => update({ note })}
                onFocus={() => setKeypadOpen(false)}
                style={styles.noteInput}
                placeholder="Add a memo"
                placeholderTextColor={colors.faint}
                multiline
              />
            </View>
          </View>

          {tags.length ? (
            <View style={styles.section}>
              <AppText weight="semibold">Tags</AppText>
              <View style={styles.tagRow}>
                {tags.map((tag) => {
                  const selected = Boolean(tag.id && draft.tagIds.includes(tag.id));
                  return (
                    <Pressable
                      key={tag.id ?? tag.name}
                      style={[styles.tagChoice, selected && styles.tagChoiceSelected]}
                      onPress={() => toggleTag(tag.id)}
                    >
                      {selected ? (
                        <Check size={13} color={colors.background} />
                      ) : (
                        <View style={[styles.tagDot, { backgroundColor: tag.color || colors.primary }]} />
                      )}
                      <AppText variant="caption" weight="medium" style={selected ? styles.tagSelectedText : styles.tagText}>
                        {tag.name}
                      </AppText>
                    </Pressable>
                  );
                })}
              </View>
            </View>
          ) : null}

          <TransactionLocationRow
            transactionId={draft.id}
            version={draft.version}
            location={draft.location}
            onDraftChange={(location) => update({ location })}
          />

          {draft.id ? <TransactionAttachments transactionId={draft.id} /> : null}
          {draft.id ? <PredictionSection key={draft.id} transactionId={draft.id} /> : null}
        </ScrollView>
      </SheetOverlay>

      {overlay === 'account' ? (
        <PickerOverlay
          title="Account"
          options={accountOptions}
          selectedId={draft.accountId}
          searchPlaceholder="Search accounts"
          onSelect={(id) => {
            if (id) update({ accountId: id });
            setOverlay(null);
          }}
          onClose={() => setOverlay(null)}
        />
      ) : null}
      {overlay === 'payee' ? (
        <PickerOverlay
          title="Payee"
          options={payeeOptions}
          selectedId={draft.payeeId}
          searchPlaceholder="Search or add a payee"
          autoFocusSearch={!draft.payeeId}
          onCreate={(name) => void createAndPickPayee(name)}
          onSelect={(id) => {
            if (id) update({ payeeId: id });
            setOverlay(null);
          }}
          onClose={() => setOverlay(null)}
        />
      ) : null}
      {overlay === 'category' ? (
        <PickerOverlay
          title="Category"
          options={categoryOptions}
          selectedId={draft.categoryId}
          searchPlaceholder="Search categories"
          clearLabel="Uncategorized"
          onSelect={(id) => {
            update({ categoryId: id });
            setOverlay(null);
          }}
          onClose={() => setOverlay(null)}
        />
      ) : null}
      {overlay === 'date' ? (
        <DatePickerOverlay
          value={draft.date}
          onSelect={(date) => {
            update({ date });
            setOverlay(null);
          }}
          onClose={() => setOverlay(null)}
        />
      ) : null}
    </Modal>
  );
}

const styles = StyleSheet.create({
  scroll: {
    gap: spacing.md,
    paddingBottom: spacing.lg
  },
  pressed: {
    opacity: 0.7
  },
  summary: {
    gap: 2
  },
  summaryTop: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: spacing.sm
  },
  avatarTile: {
    width: 48,
    height: 48,
    borderRadius: 14,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 1,
    borderColor: colors.border
  },
  statusBadge: {
    borderRadius: radii.full,
    backgroundColor: colors.successMuted,
    paddingHorizontal: spacing.md,
    paddingVertical: 4
  },
  statusBadgeReview: {
    backgroundColor: colors.primaryMuted
  },
  amountCard: {
    gap: spacing.sm,
    borderRadius: radii.lg,
    borderWidth: 1,
    borderColor: colors.heroBorder,
    backgroundColor: colors.hero,
    padding: 14,
    overflow: 'hidden'
  },
  amountCardActive: {
    borderColor: colors.primaryLight
  },
  directionToggle: {
    flexDirection: 'row',
    alignSelf: 'flex-start',
    backgroundColor: colors.heroControl,
    borderRadius: radii.full,
    padding: 3
  },
  directionOption: {
    minHeight: 30,
    paddingHorizontal: spacing.lg,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: radii.full
  },
  directionActive: {
    backgroundColor: 'rgba(255, 255, 255, 0.16)'
  },
  directionText: {
    color: colors.onHeroMuted
  },
  outflowText: {
    color: colors.heroNegative
  },
  inflowText: {
    color: colors.heroPositive
  },
  amountRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  amountSign: {
    fontSize: 26,
    lineHeight: 32
  },
  amountValue: {
    flex: 1,
    minWidth: 0
  },
  amountHint: {
    color: colors.onHeroMuted
  },
  fieldGroup: {
    borderRadius: radii.lg,
    borderWidth: 1,
    borderColor: colors.border,
    overflow: 'hidden'
  },
  fieldRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    minHeight: 54,
    paddingHorizontal: 12,
    paddingVertical: 6
  },
  fieldRowPressed: {
    backgroundColor: colors.surfaceStrong
  },
  fieldIcon: {
    width: 36,
    height: 36,
    borderRadius: 11,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  fieldText: {
    flex: 1,
    minWidth: 0
  },
  placeholder: {
    color: colors.faint
  },
  warnText: {
    color: colors.warning
  },
  divider: {
    height: StyleSheet.hairlineWidth,
    backgroundColor: colors.border,
    marginLeft: 12 + 36 + spacing.md
  },
  noteInput: {
    flex: 1,
    color: colors.text,
    fontSize: 15,
    paddingVertical: spacing.sm
  },
  section: {
    gap: spacing.sm
  },
  tagRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sm
  },
  tagChoice: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    minHeight: 32,
    paddingHorizontal: spacing.md,
    borderRadius: radii.full,
    borderWidth: 1,
    borderColor: colors.border
  },
  tagChoiceSelected: {
    backgroundColor: colors.text,
    borderColor: colors.text
  },
  tagDot: {
    width: 8,
    height: 8,
    borderRadius: 4
  },
  tagText: {
    color: colors.muted
  },
  tagSelectedText: {
    color: colors.background
  },
  footer: {
    gap: spacing.sm
  },
  actionRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  footerButton: {
    flex: 1
  },
  deleteButton: {
    width: 50,
    height: 50,
    borderRadius: 25,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.dangerMuted
  }
});
