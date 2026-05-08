import { useMemo, useState } from 'react';
import { FlatList, Modal, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { Check, CirclePlus, Search, X } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { SectionHeader } from '../../../components/SectionHeader';
import { formatCurrency, formatShortDate, getCurrentMonthKey } from '../../../utils/date';
import { colors, radii, spacing } from '../../../theme';
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
          <View style={styles.rowBetween}>
            <AppText weight="bold" style={styles.sheetTitle}>{draft.id ? 'Edit Transaction' : 'New Transaction'}</AppText>
            <Pressable onPress={onClose}><X size={22} color={colors.text} /></Pressable>
          </View>

          <TextInput value={draft.date} onChangeText={(date) => setDraft({ ...draft, date })} style={styles.input} placeholder="YYYY-MM-DD" />
          <TextInput value={draft.amount} onChangeText={(amount) => setDraft({ ...draft, amount })} style={styles.input} keyboardType="numeric" placeholder="-500" />
          <TextInput value={draft.note} onChangeText={(note) => setDraft({ ...draft, note })} style={styles.input} placeholder="Note" />

          <AppText weight="semibold">Account</AppText>
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
                <AppText weight="medium">{item.name}</AppText>
              </Pressable>
            )}
          />

          <AppText weight="semibold">Payee</AppText>
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
                <AppText weight="medium">{item.name}</AppText>
              </Pressable>
            )}
          />

          <AppText weight="semibold">Category</AppText>
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
                <AppText weight="medium">{item.name}</AppText>
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
        <SectionHeader title="Transactions" subtitle={`${transactions.length} loaded`} />
        <Pressable style={styles.iconButton} onPress={() => setDraft(createDraft())}>
          <CirclePlus size={24} color="#fff" />
        </Pressable>
      </View>

      <View style={styles.searchBox}>
        <Search size={18} color={colors.muted} />
        <TextInput value={search} onChangeText={setSearch} placeholder="Search transactions" style={styles.searchInput} />
      </View>

      <FlatList
        data={filtered}
        keyExtractor={(item) => item.id ?? `${item.date}-${item.payeeName}-${item.amount}`}
        contentContainerStyle={styles.listContent}
        showsVerticalScrollIndicator={false}
        onEndReached={() => {
          if (nextCursor && loadingMore !== 'pending') dispatch(fetchAllTransactions({ cursor: nextCursor }));
        }}
        renderItem={({ item }) => {
          const amount = (item.inflow ?? 0) || -(item.outflow ?? 0);
          const unapproved = item.status === TransactionStatus.UNAPPROVED;
          return (
            <Pressable onPress={() => setDraft(createDraft(item))}>
              <Card style={styles.txnCard}>
                <View style={styles.rowBetween}>
                  <View style={styles.txnMain}>
                    <View style={styles.rowStart}>
                      {unapproved ? <View style={styles.unapprovedDot} /> : null}
                      <AppText weight="semibold" numberOfLines={1}>{item.payeeName || 'Unknown payee'}</AppText>
                    </View>
                    <AppText muted numberOfLines={1}>{formatShortDate(item.date)} · {item.accountName}</AppText>
                    <AppText muted numberOfLines={1}>{item.categoryName ?? 'Uncategorized'}{item.note ? ` · ${item.note}` : ''}</AppText>
                  </View>
                  <View style={styles.amountBlock}>
                    <AppText weight="bold" style={amount > 0 ? styles.positive : undefined}>{formatCurrency(amount, { signed: true })}</AppText>
                    {unapproved ? (
                      <Pressable
                        style={styles.approveButton}
                        onPress={() => item.id && dispatch(updateTransactionStatus({ id: item.id, status: TransactionStatus.APPROVED }))}
                      >
                        <Check size={14} color="#fff" />
                      </Pressable>
                    ) : null}
                  </View>
                </View>
              </Card>
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
    gap: spacing.md
  },
  header: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  iconButton: {
    width: 48,
    height: 48,
    borderRadius: radii.md,
    backgroundColor: colors.primary,
    alignItems: 'center',
    justifyContent: 'center'
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
  txnCard: {
    padding: spacing.md
  },
  rowBetween: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  rowStart: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs
  },
  txnMain: {
    flex: 1
  },
  amountBlock: {
    alignItems: 'flex-end',
    gap: spacing.sm
  },
  positive: {
    color: colors.success
  },
  unapprovedDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: colors.accent
  },
  approveButton: {
    width: 28,
    height: 28,
    borderRadius: 14,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.success
  },
  modalBackdrop: {
    flex: 1,
    justifyContent: 'flex-end',
    backgroundColor: 'rgba(0,0,0,0.35)'
  },
  sheet: {
    maxHeight: '88%',
    backgroundColor: colors.surfaceStrong,
    borderTopLeftRadius: 18,
    borderTopRightRadius: 18,
    padding: spacing.lg,
    gap: spacing.md
  },
  sheetTitle: {
    fontSize: 20
  },
  input: {
    minHeight: 44,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.border,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.md,
    color: colors.text,
    backgroundColor: colors.surface
  },
  choice: {
    minHeight: 40,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    marginRight: spacing.sm,
    borderRadius: radii.sm,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.border,
    backgroundColor: colors.surface
  },
  choiceSelected: {
    borderColor: colors.primary,
    backgroundColor: colors.primaryLight
  }
});
