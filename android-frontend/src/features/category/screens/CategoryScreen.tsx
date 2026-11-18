import { useCallback, useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Pressable, RefreshControl, StyleSheet, View } from 'react-native';
import { useNavigation, useRoute, type RouteProp } from '@react-navigation/native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import type { NativeStackNavigationProp } from '@react-navigation/native-stack';
import { ArrowDown, ArrowLeftRight, ArrowUp, ChevronLeft, Plus } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AmountSheet } from '../../../components/AmountKeypad';
import { AppText } from '../../../components/AppText';
import { LetterTile, PillBars, SectionHead } from '../../../components/Blocks';
import { Card } from '../../../components/Card';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { ProgressBar } from '../../../components/ProgressBar';
import { Screen } from '../../../components/Screen';
import type { RootStackParamList } from '../../../navigation/types';
import { chartSeries, colors, radii, spacing } from '../../../theme';
import { apiClient } from '../../../utils/api';
import type { PaginationResponse } from '../../../utils/constants';
import { formatCurrency, formatMonthName, formatRelativeDay, formatSignedCurrency, shiftMonth, todayKey } from '../../../utils/date';
import type { IncomeExpenseReport } from '../../reports/types';
import { TransactionEditor, createDraft, type TxnDraft } from '../../transactions/components/TransactionEditor';
import { TransactionStatus, type Transaction } from '../../transactions/types';
import { fetchAllCategoryGroups, fetchInflowAmount, updateCategoryBudget } from '../store/categorySlice';

type Nav = NativeStackNavigationProp<RootStackParamList>;

const HISTORY_MONTHS = 6;

function monthRange(month: string) {
  const [year, monthNumber] = month.split('-').map(Number);
  const lastDay = new Date(year, monthNumber, 0).getDate();
  return { startDate: `${month}-01`, endDate: `${month}-${String(lastDay).padStart(2, '0')}` };
}

/** Spending per month for one category, from the income/expense report. */
function useCategoryHistory(categoryId: string, month: string, revision: number) {
  const [points, setPoints] = useState<{ month: string; value: number }[]>([]);
  useEffect(() => {
    let cancelled = false;
    apiClient
      .get<IncomeExpenseReport>(`reports/income-expense?startMonth=${shiftMonth(month, -(HISTORY_MONTHS - 1))}&endMonth=${month}`)
      .then((report) => {
        if (cancelled) return;
        const row = report.expense.groups.flatMap((group) => group.categories).find((category) => category.categoryId === categoryId);
        setPoints(report.months.filter((m) => m <= month).map((m) => ({ month: m, value: row?.amounts[m] ?? 0 })));
      })
      .catch(() => !cancelled && setPoints([]));
    return () => {
      cancelled = true;
    };
  }, [categoryId, month, revision]);
  return points;
}

function useCategoryTransactions(categoryId: string, month: string, revision: number) {
  const [transactions, setTransactions] = useState<Transaction[] | null>(null);
  useEffect(() => {
    let cancelled = false;
    const { startDate, endDate } = monthRange(month);
    const params = new URLSearchParams({ startDate, endDate, limit: '100' });
    params.set('categoryId[]', categoryId);
    apiClient
      .get<PaginationResponse<Transaction[]>>(`transactions/normalized?${params.toString()}`)
      .then((response) => !cancelled && setTransactions(response.data ?? []))
      .catch(() => !cancelled && setTransactions([]));
    return () => {
      cancelled = true;
    };
  }, [categoryId, month, revision]);
  return transactions;
}

/**
 * One category for the budget's selected month: spent against assigned, what's
 * available, six months of spending, and the month's transactions.
 */
export function CategoryScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation<Nav>();
  const { id } = useRoute<RouteProp<RootStackParamList, 'Category'>>().params;
  const month = useAppSelector((state) => state.budgets.selectedMonth);
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  const refreshing = useAppSelector((state) => state.categories.loading === 'pending');
  const [revision, setRevision] = useState(0);
  const [assigning, setAssigning] = useState(false);
  const [draft, setDraft] = useState<TxnDraft | null>(null);
  const [editorKey, setEditorKey] = useState(0);
  const insets = useSafeAreaInsets();

  const found = useMemo(() => {
    const visible = groups.filter((group) => !group.isSystem);
    for (const [index, group] of visible.entries()) {
      const category = group.categories.find((item) => item.id === id);
      if (category) return { group, category, color: chartSeries[index % chartSeries.length] };
    }
    return null;
  }, [groups, id]);

  const history = useCategoryHistory(id, month, revision);
  const transactions = useCategoryTransactions(id, month, revision);

  const refresh = useCallback(() => {
    dispatch(fetchAllCategoryGroups(month));
    dispatch(fetchInflowAmount());
    setRevision((current) => current + 1);
  }, [dispatch, month]);

  if (!found) {
    return (
      <Screen>
        <Header title="Category" onBack={() => navigation.goBack()} />
        <AppText muted>This category isn't in the current budget month.</AppText>
      </Screen>
    );
  }

  const { group, category, color } = found;
  const budgeted = category.budgeted?.[month] ?? 0;
  const spent = Math.max(-(category.activity?.[month] ?? 0), 0);
  const available = category.balance?.[month] ?? 0;
  const hasBudget = budgeted > 0;
  const over = available < 0;
  const percent = hasBudget ? (spent / budgeted) * 100 : spent > 0 ? 100 : 0;

  const previous = history.find((point) => point.month === shiftMonth(month, -1))?.value ?? 0;
  const change = previous > 0 ? ((spent - previous) / previous) * 100 : null;
  const today = todayKey();

  return (
    <View style={styles.root}>
      <Screen style={styles.screen} refreshControl={<RefreshControl refreshing={refreshing} onRefresh={refresh} tintColor={colors.primary} />}>
        <Header title="Category" onBack={() => navigation.goBack()} />

        <View style={styles.titleBlock}>
          <View style={styles.titleTop}>
            <LetterTile name={category.name} color={color} size={48} />
            {change !== null && Math.round(change) !== 0 ? (
              <View style={styles.changeBlock}>
                <View style={[styles.changeBadge, change > 0 ? styles.changeUp : styles.changeDown]}>
                  {change > 0 ? <ArrowUp size={12} color={colors.danger} /> : <ArrowDown size={12} color={colors.success} />}
                  <AppText variant="caption" weight="semibold" tone={change > 0 ? 'danger' : 'success'}>
                    {`${Math.abs(Math.round(change))}%`}
                  </AppText>
                </View>
                <AppText variant="caption" tone="faint">
                  vs {formatMonthName(shiftMonth(month, -1), 'short')}
                </AppText>
              </View>
            ) : null}
          </View>
          <AppText variant="title" numberOfLines={2}>
            {category.name}
          </AppText>
          <AppText variant="caption" muted>
            {`${group.name} · ${formatMonthName(month)}`}
          </AppText>
        </View>

        <Card style={styles.card}>
          <View style={styles.row}>
            <AppText variant="heading" tabular style={styles.grow}>
              {formatCurrency(spent)}
            </AppText>
            <AppText tone="faint" tabular>
              {hasBudget ? formatCurrency(budgeted) : 'Nothing assigned'}
            </AppText>
          </View>
          <ProgressBar percent={percent} height={6} color={over ? colors.danger : color} />
          <View style={styles.row}>
            <AppText variant="caption" tone={over ? 'danger' : 'muted'} style={styles.grow}>
              {over ? `Overspent by ${formatCurrency(-available)}` : `${formatCurrency(available)} available`}
            </AppText>
            {hasBudget ? (
              <AppText variant="caption" weight="bold" tabular>
                {`${Math.round(percent)}%`}
              </AppText>
            ) : null}
          </View>
          <PillBars title="Spending analytics" points={history} />
        </Card>

        <SectionHead
          title="Transactions"
          onViewAll={() =>
            navigation.navigate('Main', { screen: 'Transactions', params: { categoryId: id, categoryName: category.name, month } })
          }
        />
        <Card style={styles.card}>
          {transactions === null ? (
            <ActivityIndicator color={colors.primary} />
          ) : transactions.length === 0 ? (
            <AppText variant="caption" tone="faint">
              Nothing in {formatMonthName(month)} yet.
            </AppText>
          ) : (
            transactions.map((txn) => {
              const net = (txn.inflow ?? 0) - (txn.outflow ?? 0);
              const transfer = Boolean(txn.transferAccountId);
              const name = txn.payeeName || 'Unknown payee';
              return (
                <Pressable
                  key={txn.id}
                  style={({ pressed }) => [styles.txnCard, pressed && styles.pressed]}
                  onPress={() => {
                    setEditorKey((key) => key + 1);
                    setDraft(createDraft(txn));
                  }}
                >
                  <View style={styles.txnMain}>
                    <View style={styles.txnTile}>
                      {transfer ? <ArrowLeftRight size={18} color={colors.muted} /> : <InitialAvatar name={name} size={32} />}
                    </View>
                    <View style={styles.grow}>
                      <AppText weight="semibold" numberOfLines={1}>
                        {name}
                      </AppText>
                      <AppText variant="caption" muted numberOfLines={1}>
                        {txn.note ? `${txn.accountName} · ${txn.note}` : txn.accountName}
                      </AppText>
                    </View>
                    <View style={styles.end}>
                      <AppText weight="bold" tabular tone={net > 0 && !transfer ? 'success' : undefined}>
                        {formatSignedCurrency(net, !transfer)}
                      </AppText>
                      <AppText variant="caption" tone={txn.status === TransactionStatus.UNAPPROVED ? 'primary' : 'faint'}>
                        {txn.status === TransactionStatus.UNAPPROVED ? 'Needs review' : formatRelativeDay(txn.date, today)}
                      </AppText>
                    </View>
                  </View>
                </Pressable>
              );
            })
          )}
        </Card>
      </Screen>

      <View style={[styles.ctaWrap, { bottom: Math.max(insets.bottom, spacing.md) + spacing.sm }]} pointerEvents="box-none">
        <Pressable
          accessibilityRole="button"
          style={({ pressed }) => [styles.cta, pressed && styles.pressed]}
          onPress={() => setAssigning(true)}
        >
          <Plus size={18} color={colors.onHero} />
          <AppText weight="semibold" style={styles.ctaText}>
            {hasBudget ? 'Change assigned' : 'Assign money'}
          </AppText>
        </Pressable>
      </View>

      <AmountSheet
        visible={assigning}
        title={`Assign to ${category.name}`}
        subtitle={`${formatCurrency(available)} available · type a sum like 1200+300`}
        initialValue={budgeted}
        onClose={() => setAssigning(false)}
        onSave={(next) => {
          if (next !== budgeted) dispatch(updateCategoryBudget({ categoryId: id, month, budgeted: next }));
          setAssigning(false);
        }}
      />

      {draft ? (
        <TransactionEditor
          key={editorKey}
          initialDraft={draft}
          onClose={() => setDraft(null)}
          onSaved={() => {
            setDraft(null);
            refresh();
          }}
        />
      ) : null}
    </View>
  );
}

function Header({ title, onBack }: { title: string; onBack: () => void }) {
  return (
    <View style={styles.header}>
      <Pressable accessibilityRole="button" accessibilityLabel="Go back" style={({ pressed }) => [styles.back, pressed && styles.pressed]} onPress={onBack}>
        <ChevronLeft size={20} color={colors.text} />
      </Pressable>
      <AppText weight="semibold" style={styles.headerTitle}>
        {title}
      </AppText>
      <View style={styles.back} />
    </View>
  );
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.background
  },
  screen: {
    gap: spacing.md
  },
  grow: {
    flex: 1,
    minWidth: 0
  },
  pressed: {
    opacity: 0.7
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center'
  },
  headerTitle: {
    flex: 1,
    textAlign: 'center'
  },
  back: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surface
  },
  titleBlock: {
    gap: 2
  },
  titleTop: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: spacing.sm
  },
  changeBlock: {
    alignItems: 'flex-end',
    gap: 2
  },
  changeBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 3,
    borderRadius: radii.full,
    paddingHorizontal: spacing.sm + 2,
    paddingVertical: 3
  },
  changeUp: {
    backgroundColor: colors.dangerMuted
  },
  changeDown: {
    backgroundColor: colors.successMuted
  },
  card: {
    gap: 10
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  end: {
    alignItems: 'flex-end',
    maxWidth: '45%'
  },
  txnCard: {
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: colors.border
  },
  txnMain: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingHorizontal: 10,
    paddingVertical: spacing.sm
  },
  txnTile: {
    width: 40,
    height: 40,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  ctaWrap: {
    position: 'absolute',
    left: spacing.lg,
    right: spacing.lg
  },
  cta: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.sm,
    height: 52,
    borderRadius: 26,
    backgroundColor: colors.hero,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.heroBorder,
    elevation: 10,
    shadowColor: colors.shadow,
    shadowOffset: { width: 0, height: 6 },
    shadowOpacity: 0.25,
    shadowRadius: 14
  },
  ctaText: {
    color: colors.onHero
  }
});
