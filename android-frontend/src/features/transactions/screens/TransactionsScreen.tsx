import { useMemo, useState } from 'react';
import { FlatList, Modal, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { Check, Plus, ReceiptText, Search, X } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { EmptyState } from '../../../components/EmptyState';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { formatCurrency, formatShortDate } from '../../../utils/date';
import { colors, radii, spacing, tabBarClearance } from '../../../theme';
import type { Transaction, TransactionDTO } from '../types';
import { TransactionStatus } from '../types';
import { createTransaction, fetchAllTransactions, updateTransaction, updateTransactionStatus } from '../store/transactionSlice';

type TxnDraft = {
  id?: string;
  accountId: string;
  payeeId: string;
  categoryId: string | null;
  date: string;
  amount: string;
  note: string;
  status?: TransactionStatus;
};

function createDraft(txn?: Transaction): TxnDraft {
  const normalizedAmount = txn?.amount ?? ((txn?.inflow ?? 0) || -(txn?.outflow ?? 0));
  return {
    id: txn?.id,
    accountId: txn?.accountId ?? '',
    payeeId: txn?.payeeId ?? '',
    categoryId: txn?.categoryId ?? null,
    date: txn?.date ?? new Date().toISOString().slice(0, 10),
    amount: txn ? String(normalizedAmount) : '',
    note: txn?.note ?? '',
    status: txn?.status
  };
}

function TransactionEditor({
  draft,
  setDraft,
  onClose,
  onSave
}: {
  draft: TxnDraft | null;
  setDraft: (draft: TxnDraft) => void;
  onClose: () => void;
  onSave: () => void;
}) {
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const payees = useAppSelector((state) => state.payees.allPayees);
  const categories = useAppSelector((state) => state.categories.allCategoryGroups.flatMap((group) => group.categories));
  if (!draft) return null;

  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose}>
      <View style={styles.modalBackdrop}>
        <View style={styles.sheet}>
          <View style={styles.sheetHandle} />
          <View style={styles.rowBetween}>
            <AppText variant="title" style={styles.sheetTitle}>{draft.id ? 'Edit transaction' : 'New transaction'}</AppText>
            <Pressable style={({ pressed }) => [styles.closeButton, pressed && styles.pressed]} onPress={onClose}>
              <X size={18} color={colors.muted} />
            </Pressable>
          </View>

          <TextInput
            value={draft.date}
            onChangeText={(date) => setDraft({ ...draft, date })}
            style={styles.input}
            placeholder="YYYY-MM-DD"
            placeholderTextColor={colors.faint}
          />
          <TextInput
            value={draft.amount}
            onChangeText={(amount) => setDraft({ ...draft, amount })}
            style={styles.input}
            keyboardType="numeric"
            placeholder="-500"
            placeholderTextColor={colors.faint}
          />
          <TextInput
            value={draft.note}
            onChangeText={(note) => setDraft({ ...draft, note })}
            style={styles.input}
            placeholder="Note"
            placeholderTextColor={colors.faint}
          />

          <AppText variant="label" tone="faint">Account</AppText>
          <FlatList
            horizontal
            data={accounts}
            keyExtractor={(item) => item.id ?? item.name}
            showsHorizontalScrollIndicator={false}
            renderItem={({ item }) => (
              <Pressable
                style={[styles.choice, draft.accountId === item.id && styles.choiceSelected]}
                onPress={() => item.id && setDraft({ ...draft, accountId: item.id })}
              >
                <AppText variant="caption" weight="medium" style={draft.accountId === item.id ? styles.choiceSelectedText : styles.choiceText}>
                  {item.name}
                </AppText>
              </Pressable>
            )}
          />

          <AppText variant="label" tone="faint">Payee</AppText>
          <FlatList
            horizontal
            data={payees}
            keyExtractor={(item) => item.id ?? item.name}
            showsHorizontalScrollIndicator={false}
            renderItem={({ item }) => (
              <Pressable
                style={[styles.choice, draft.payeeId === item.id && styles.choiceSelected]}
                onPress={() => item.id && setDraft({ ...draft, payeeId: item.id })}
              >
                <AppText variant="caption" weight="medium" style={draft.payeeId === item.id ? styles.choiceSelectedText : styles.choiceText}>
                  {item.name}
                </AppText>
              </Pressable>
            )}
          />

          <AppText variant="label" tone="faint">Category</AppText>
          <FlatList
            horizontal
            data={categories}
            keyExtractor={(item) => item.id ?? item.name}
            showsHorizontalScrollIndicator={false}
            renderItem={({ item }) => (
              <Pressable
                style={[styles.choice, draft.categoryId === item.id && styles.choiceSelected]}
                onPress={() => item.id && setDraft({ ...draft, categoryId: item.id })}
              >
                <AppText variant="caption" weight="medium" style={draft.categoryId === item.id ? styles.choiceSelectedText : styles.choiceText}>
                  {item.name}
                </AppText>
              </Pressable>
            )}
          />

          <Button onPress={onSave}>Save</Button>
        </View>
      </View>
    </Modal>
  );
}

export function TransactionsScreen() {
  const dispatch = useAppDispatch();
  const selectedBudget = useAppSelector((state) => state.budgets.selectedBudget);
  const { transactions, nextCursor, loadingMore } = useAppSelector((state) => state.transactions);
  const [search, setSearch] = useState('');
  const [draft, setDraft] = useState<TxnDraft | null>(null);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return transactions;
    return transactions.filter((txn) =>
      [txn.payeeName, txn.accountName, txn.categoryName ?? '', txn.note ?? ''].some((value) => value.toLowerCase().includes(q))
    );
  }, [search, transactions]);

  const save = async () => {
    if (!draft || !selectedBudget?.id) return;
    const amount = Number(draft.amount);
    if (!draft.accountId || !draft.payeeId || !Number.isFinite(amount)) return;
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
      tagIds: []
    };
    if (draft.id) await dispatch(updateTransaction(payload)).unwrap();
    else await dispatch(createTransaction(payload)).unwrap();
    setDraft(null);
    dispatch(fetchAllTransactions());
  };

  return (
    <Screen scroll={false} style={styles.screen}>
      <View style={styles.header}>
        <View style={styles.headerText}>
          <AppText variant="title">Transactions</AppText>
          <AppText variant="caption" muted>{transactions.length} loaded</AppText>
        </View>
        <Pressable style={({ pressed }) => [styles.addButton, pressed && styles.pressed]} onPress={() => setDraft(createDraft())}>
          <Plus size={22} color={colors.onPrimary} />
        </Pressable>
      </View>

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
        ListEmptyComponent={
          <EmptyState
            icon={<ReceiptText size={26} color={colors.primary} />}
            title="No transactions"
            body="Transactions from your accounts and Gmail imports will show up here."
          />
        }
        onEndReached={() => {
          if (nextCursor && loadingMore !== 'pending') dispatch(fetchAllTransactions({ cursor: nextCursor }));
        }}
        renderItem={({ item }) => {
          const amount = (item.inflow ?? 0) || -(item.outflow ?? 0);
          const unapproved = item.status === TransactionStatus.UNAPPROVED;
          return (
            <Pressable style={({ pressed }) => [styles.txnRow, pressed && styles.rowPressed]} onPress={() => setDraft(createDraft(item))}>
              <InitialAvatar name={item.payeeName || '?'} size={42} />
              <View style={styles.txnMain}>
                <AppText weight="medium" numberOfLines={1}>{item.payeeName || 'Unknown payee'}</AppText>
                <AppText variant="caption" muted numberOfLines={1}>
                  {formatShortDate(item.date)} · {item.categoryName ?? 'Uncategorized'}
                </AppText>
                {item.note ? <AppText variant="caption" tone="faint" numberOfLines={1}>{item.note}</AppText> : null}
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

      <TransactionEditor draft={draft} setDraft={setDraft} onClose={() => setDraft(null)} onSave={() => void save()} />
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
  txnRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    backgroundColor: colors.surface,
    borderRadius: radii.md,
    padding: spacing.md
  },
  rowPressed: {
    backgroundColor: colors.surfaceStrong
  },
  txnMain: {
    flex: 1,
    gap: 1
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
    maxHeight: '88%',
    backgroundColor: colors.surface,
    borderTopLeftRadius: radii.xl,
    borderTopRightRadius: radii.xl,
    padding: spacing.xl,
    paddingTop: spacing.md,
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
  choice: {
    minHeight: 36,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    marginRight: spacing.sm,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong
  },
  choiceSelected: {
    backgroundColor: colors.primaryMuted
  },
  choiceText: {
    color: colors.muted
  },
  choiceSelectedText: {
    color: colors.primary
  }
});
