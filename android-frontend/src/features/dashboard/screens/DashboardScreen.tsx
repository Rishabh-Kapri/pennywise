import { useCallback, useEffect, useMemo, useState } from 'react';
import { Image, Pressable, RefreshControl, StyleSheet, View } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { useNavigation } from '@react-navigation/native';
import type { NativeStackNavigationProp } from '@react-navigation/native-stack';
import {
  AlertCircle,
  ArrowDown,
  ArrowDownLeft,
  ArrowLeftRight,
  ArrowRight,
  ArrowUp,
  ArrowUpRight,
  Check,
  CheckCircle2,
  Landmark,
  Sparkles,
  TriangleAlert,
  UserRound
} from 'lucide-react-native';
import type { RootStackParamList } from '../../../navigation/types';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { LineChart, Ring, Segmented } from '../../../components/charts';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { Screen } from '../../../components/Screen';
import { chartSeries, colors, radii, spacing } from '../../../theme';
import {
  addDays,
  formatCompactCurrency,
  formatCurrency,
  formatDayMonth,
  formatMonthName,
  formatRelativeDay,
  formatSignedCurrency,
  shiftMonth,
  todayKey
} from '../../../utils/date';
import { fetchAllAccounts } from '../../accounts/store/accountSlice';
import { fetchAllCategoryGroups, fetchInflowAmount } from '../../category/store/categorySlice';
import { RecentDocumentsCard } from '../../documents/components/RecentDocumentsCard';
import { TransactionStatus, type Transaction } from '../../transactions/types';
import {
  ACTIVITY_DAYS,
  buildBalanceHistory,
  fetchDashboardActivity,
  fetchDashboardCashFlow,
  selectActivityWindow,
  selectDashboardAccounts,
  selectMonthCashFlow,
  selectNeedsReviewCount,
  selectSpendingSummary,
  type SpendingGroup
} from '../store/dashboardSlice';

type Nav = NativeStackNavigationProp<RootStackParamList>;

function greeting() {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 17) return 'Good afternoon';
  return 'Good evening';
}

const ACCOUNT_TYPE: Record<string, string> = {
  checking: 'Checking',
  savings: 'Savings',
  creditCard: 'Credit card',
  asset: 'Asset'
};

const excludedKey = (budgetId: string) => `pennywise.dashboard.excludedAccounts.${budgetId}`;

/** Accounts unticked from the balance, per budget. Stores the left-out ones so a new account counts by default. */
function useExcludedAccounts(budgetId?: string) {
  const [excluded, setExcluded] = useState<Set<string>>(new Set());

  useEffect(() => {
    setExcluded(new Set());
    if (!budgetId) return;
    let cancelled = false;
    AsyncStorage.getItem(excludedKey(budgetId))
      .then((raw) => {
        const ids: unknown = JSON.parse(raw ?? '[]');
        if (!cancelled && Array.isArray(ids)) setExcluded(new Set(ids.filter((id): id is string => typeof id === 'string')));
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [budgetId]);

  const toggle = useCallback(
    (accountId: string) => {
      setExcluded((current) => {
        const next = new Set(current);
        if (next.has(accountId)) next.delete(accountId);
        else next.add(accountId);
        if (budgetId) void AsyncStorage.setItem(excludedKey(budgetId), JSON.stringify([...next]));
        return next;
      });
    },
    [budgetId]
  );

  return [excluded, toggle] as const;
}

function CardTitle({ title, right }: { title: string; right?: React.ReactNode }) {
  return (
    <View style={styles.cardHead}>
      <AppText variant="heading" style={styles.grow}>
        {title}
      </AppText>
      {right}
    </View>
  );
}

// ── Balance ──

const RANGES = [
  { value: '1m', label: '1M', days: 30 },
  { value: '3m', label: '3M', days: 91 },
  { value: '6m', label: '6M', days: ACTIVITY_DAYS }
] as const;
type Range = (typeof RANGES)[number]['value'];

function BalanceCard() {
  const accounts = useAppSelector(selectDashboardAccounts);
  const activity = useAppSelector((state) => state.dashboard.activity);
  const activityWindow = useAppSelector(selectActivityWindow);
  const budgetId = useAppSelector((state) => state.budgets.selectedBudget?.id);
  const [excluded, toggle] = useExcludedAccounts(budgetId);
  const [range, setRange] = useState<Range>('3m');
  const [showAccounts, setShowAccounts] = useState(false);
  const [selected, setSelected] = useState<number | null>(null);
  const today = todayKey();

  const { ids, total } = useMemo(() => {
    const included = new Set<string>();
    let sum = 0;
    for (const account of accounts) {
      if (!account.id || excluded.has(account.id)) continue;
      included.add(account.id);
      sum += account.balance ?? 0;
    }
    return { ids: included, total: sum };
  }, [accounts, excluded]);

  const history = useMemo(() => {
    if (!activityWindow.start || !ids.size) return [];
    const days = RANGES.find((option) => option.value === range)?.days ?? ACTIVITY_DAYS;
    const rangeStart = addDays(today, -days);
    const start = rangeStart > activityWindow.reliableStart ? rangeStart : activityWindow.reliableStart;
    return buildBalanceHistory(total, activity, ids, start, today);
  }, [activity, activityWindow, ids, range, today, total]);

  // a point per day is too fine for a phone-width line; sample down to ~45
  const points = useMemo(() => {
    const step = Math.max(1, Math.ceil(history.length / 45));
    return history
      .filter((_, i) => i % step === 0 || i === history.length - 1)
      .map((point) => ({ key: point.date, label: formatDayMonth(point.date), values: { balance: point.balance } }));
  }, [history]);

  const first = history[0];
  const change = first ? total - first.balance : 0;
  const pick = selected != null ? points[selected] : null;

  return (
    <Card style={styles.card}>
      <CardTitle
        title="Balance"
        right={<Segmented options={RANGES} value={range} onChange={(value) => { setRange(value); setSelected(null); }} />}
      />
      <View>
        <AppText variant="caption" muted>
          {pick
            ? pick.label
            : ids.size === accounts.length
              ? `Across ${accounts.length === 1 ? 'your account' : `all ${accounts.length} accounts`}`
              : `Across ${ids.size} of ${accounts.length} accounts`}
        </AppText>
        <AppText variant="display" tabular>
          {formatSignedCurrency(pick ? pick.values.balance : total)}
        </AppText>
        {first && history.length > 1 && !pick ? (
          <View style={styles.inline}>
            {Math.round(change) === 0 ? (
              <AppText variant="caption" muted>
                No change
              </AppText>
            ) : (
              <>
                {change > 0 ? <ArrowUp size={13} color={colors.success} /> : <ArrowDown size={13} color={colors.danger} />}
                <AppText variant="caption" weight="semibold" tone={change > 0 ? 'success' : 'danger'} tabular>
                  {formatSignedCurrency(change, true)}
                </AppText>
              </>
            )}
            <AppText variant="caption" tone="faint">
              {' '}
              since {formatDayMonth(first.date)}
            </AppText>
          </View>
        ) : null}
      </View>
      {points.length > 1 ? (
        <LineChart
          points={points}
          series={[{ key: 'balance', label: 'Balance', color: chartSeries[0] }]}
          height={140}
          selected={selected}
          onSelect={(index) => setSelected((current) => (current === index ? null : index))}
        />
      ) : ids.size === 0 ? (
        <AppText variant="caption" tone="faint">
          Tick an account to chart its balance.
        </AppText>
      ) : null}

      <Pressable style={styles.linkRow} onPress={() => setShowAccounts((open) => !open)}>
        <AppText variant="caption" weight="semibold" tone="primary">
          {showAccounts ? 'Hide accounts' : `${accounts.length} accounts`}
        </AppText>
      </Pressable>
      {showAccounts
        ? accounts.map((account) => {
            const included = Boolean(account.id && ids.has(account.id));
            const balance = account.balance ?? 0;
            return (
              <Pressable
                key={account.id}
                style={styles.accountRow}
                onPress={() => account.id && toggle(account.id)}
                accessibilityRole="checkbox"
                accessibilityState={{ checked: included }}
              >
                <View style={[styles.check, included && styles.checkOn]}>
                  {included ? <Check size={12} color={colors.onPrimary} /> : null}
                </View>
                <View style={styles.grow}>
                  <AppText numberOfLines={1}>{account.name}</AppText>
                  <AppText variant="caption" tone="faint">
                    {ACCOUNT_TYPE[account.type] ?? 'Account'}
                  </AppText>
                </View>
                <AppText weight="semibold" tabular tone={balance < 0 ? 'danger' : undefined}>
                  {formatSignedCurrency(balance)}
                </AppText>
              </Pressable>
            );
          })
        : null}
    </Card>
  );
}

// ── Cash flow ──

function Change({ current, previous, upIsGood, against }: { current: number; previous: number | null; upIsGood: boolean; against: string }) {
  if (previous === null) return null;
  const diff = current - previous;
  if (Math.round(diff) === 0) {
    return (
      <AppText variant="caption" tone="faint">
        Same as {against}
      </AppText>
    );
  }
  const up = diff > 0;
  const good = up === upIsGood;
  return (
    <View style={styles.inline}>
      {up ? <ArrowUp size={12} color={good ? colors.success : colors.danger} /> : <ArrowDown size={12} color={good ? colors.success : colors.danger} />}
      <AppText variant="caption" weight="semibold" tone={good ? 'success' : 'danger'}>
        {formatCompactCurrency(Math.abs(diff))}
      </AppText>
      <AppText variant="caption" tone="faint">
        {' '}
        vs {against}
      </AppText>
    </View>
  );
}

function CashFlowCard() {
  const flow = useAppSelector(selectMonthCashFlow);
  if (!flow) return null;
  const prevName = formatMonthName(shiftMonth(flow.month, -1), 'short');
  const spentShare = flow.income > 0 ? flow.spending / flow.income : null;
  const overspent = flow.net < 0;

  return (
    <Card style={styles.card}>
      <CardTitle
        title="Cash flow"
        right={
          <AppText variant="caption" muted>
            {formatMonthName(flow.month)}
          </AppText>
        }
      />
      <View style={styles.flowRow}>
        <View style={styles.flowCell}>
          <View style={styles.inline}>
            <View style={[styles.flowIcon, { backgroundColor: colors.successMuted }]}>
              <ArrowDownLeft size={14} color={colors.success} />
            </View>
            <AppText variant="caption" muted>
              Income
            </AppText>
          </View>
          <AppText variant="heading" tabular tone="success">
            {formatSignedCurrency(flow.income, true)}
          </AppText>
          <Change current={flow.income} previous={flow.prevIncome} upIsGood against={prevName} />
        </View>
        <View style={styles.flowCell}>
          <View style={styles.inline}>
            <View style={[styles.flowIcon, { backgroundColor: colors.surfaceStrong }]}>
              <ArrowUpRight size={14} color={colors.muted} />
            </View>
            <AppText variant="caption" muted>
              Spending
            </AppText>
          </View>
          <AppText variant="heading" tabular>
            {formatSignedCurrency(-flow.spending)}
          </AppText>
          <Change current={flow.spending} previous={flow.prevSpending} upIsGood={false} against={prevName} />
        </View>
      </View>

      <View style={styles.netBox}>
        <View style={styles.cardHead}>
          <AppText variant="caption" muted style={styles.grow}>
            {overspent ? 'Overspent' : 'Left over'}
          </AppText>
          <AppText weight="semibold" tabular tone={overspent ? 'danger' : undefined}>
            {formatCurrency(Math.abs(flow.net))}
          </AppText>
        </View>
        <View style={styles.meter}>
          <View
            style={[
              styles.meterFill,
              spentShare !== null && spentShare > 1 && styles.meterOver,
              { width: `${spentShare === null ? 0 : Math.min(spentShare, 1) * 100}%` }
            ]}
          />
        </View>
        <AppText variant="caption" tone="faint">
          {spentShare === null
            ? flow.spending > 0
              ? 'No income recorded this month'
              : 'Nothing in or out yet'
            : spentShare <= 1
              ? `${Math.round(spentShare * 100)}% of income spent`
              : `Spent ${formatCurrency(flow.spending - flow.income)} more than came in`}
        </AppText>
      </View>
      <AppText variant="caption" tone="faint">
        {flow.avgMonths > 0
          ? `Monthly average, last ${flow.avgMonths === 1 ? 'month' : `${flow.avgMonths} months`}: +${formatCurrency(flow.avgIncome)} in · −${formatCurrency(flow.avgSpending)} out`
          : 'Averages appear after your first full month.'}
      </AppText>
    </Card>
  );
}

// ── Spending ──

function CategoryTile({ name, spent, budgeted }: { name: string; spent: number; budgeted: number }) {
  const hasBudget = budgeted > 0;
  const over = hasBudget && spent > budgeted;
  const percent = hasBudget ? (spent / budgeted) * 100 : 0;
  return (
    <View style={styles.tile}>
      <Ring percent={percent} size={58} stroke={6} color={over ? colors.danger : hasBudget ? colors.primary : colors.surfaceTertiary}>
        <AppText variant="caption" weight="semibold" tabular style={styles.ringText}>
          {hasBudget ? `${Math.round(percent)}%` : '—'}
        </AppText>
      </Ring>
      <AppText variant="caption" numberOfLines={1} style={styles.center}>
        {name}
      </AppText>
      <AppText variant="caption" weight="semibold" tabular>
        {formatCurrency(spent)}
      </AppText>
      <AppText variant="caption" tone={over ? 'danger' : 'faint'} numberOfLines={1} style={styles.tileStatus}>
        {over
          ? `${formatCurrency(spent - budgeted)} over`
          : !hasBudget
            ? 'Not budgeted'
            : spent === budgeted
              ? 'Fully spent'
              : `${formatCurrency(budgeted - spent)} left`}
      </AppText>
    </View>
  );
}

function SpendingCard({ onUncategorized }: { onUncategorized: () => void }) {
  const summary = useAppSelector(selectSpendingSummary);
  const activity = useAppSelector((state) => state.dashboard.activity);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  if (!summary) return null;
  const month = summary.month;
  const uncategorized = activity.filter(
    (txn) => txn.date.startsWith(month) && !txn.categoryId && !txn.transferAccountId
  ).length;
  const groups = summary.groups;
  const selected: SpendingGroup | undefined = groups.find((group) => group.id === selectedId) ?? groups[0];

  return (
    <Card style={styles.card}>
      <CardTitle
        title="Spending"
        right={
          summary.total > 0 ? (
            <AppText variant="caption" muted tabular>
              <AppText variant="caption" weight="semibold">
                {formatCurrency(summary.total)}
              </AppText>
              {summary.budgeted > 0 ? ` of ${formatCurrency(summary.budgeted)}` : ''}
            </AppText>
          ) : null
        }
      />
      {!selected ? (
        <AppText variant="caption" tone="faint">
          No spending in {formatMonthName(month)} yet.
        </AppText>
      ) : (
        <>
          {groups.slice(0, 6).map((group) => {
            const share = summary.total > 0 ? (group.spent / summary.total) * 100 : 0;
            const active = group.id === selected.id;
            return (
              <Pressable
                key={group.id}
                style={[styles.groupRow, active && styles.groupRowActive]}
                onPress={() => setSelectedId(group.id)}
              >
                <View style={styles.cardHead}>
                  <AppText weight={active ? 'semibold' : 'medium'} numberOfLines={1} style={styles.grow}>
                    {group.name}
                  </AppText>
                  <AppText weight="semibold" tabular>
                    {formatCurrency(group.spent)}
                  </AppText>
                </View>
                <View style={styles.bar}>
                  <View style={[styles.barFill, { width: `${Math.max(share, 1.5)}%` }]} />
                </View>
              </Pressable>
            );
          })}
          <View style={styles.tilesHead}>
            <AppText weight="semibold" style={styles.grow} numberOfLines={1}>
              {selected.name}
            </AppText>
            <AppText variant="caption" tone="faint">
              {selected.budgeted > 0 ? `of ${formatCurrency(selected.budgeted)} assigned` : 'Nothing assigned'}
            </AppText>
          </View>
          <View style={styles.tiles}>
            {selected.categories.map((category) => (
              <CategoryTile key={category.id} name={category.name} spent={category.spent} budgeted={category.budgeted} />
            ))}
          </View>
        </>
      )}
      {uncategorized > 0 ? (
        <Pressable style={styles.attention} onPress={onUncategorized}>
          <AlertCircle size={16} color={colors.warning} />
          <AppText variant="caption" weight="semibold" style={[styles.grow, styles.warnText]}>
            {uncategorized} uncategorized {uncategorized === 1 ? 'transaction' : 'transactions'} in {formatMonthName(month)}
          </AppText>
          <ArrowRight size={14} color={colors.warning} />
        </Pressable>
      ) : null}
    </Card>
  );
}

// ── Recent activity ──

const FILTERS = [
  { value: 'all', label: 'All' },
  { value: 'out', label: 'Spending' },
  { value: 'in', label: 'Income' }
] as const;
type Filter = (typeof FILTERS)[number]['value'];

function RecentActivity({ onSeeAll }: { onSeeAll: () => void }) {
  const activity = useAppSelector((state) => state.dashboard.activity);
  const needsReview = useAppSelector(selectNeedsReviewCount);
  const inflowCategoryId = useAppSelector((state) => state.budgets.selectedBudget?.metadata?.inflowCategoryId);
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const [filter, setFilter] = useState<Filter>('all');
  const today = todayKey();

  const days = useMemo(() => {
    const matches = (txn: Transaction) => {
      if (filter === 'all') return true;
      if (txn.transferAccountId) return false;
      return filter === 'in' ? (txn.inflow ?? 0) > 0 : (txn.outflow ?? 0) > 0;
    };
    const grouped: { date: string; transactions: Transaction[] }[] = [];
    for (const txn of activity.filter((t) => t.date <= today && matches(t)).slice(0, 12)) {
      const last = grouped[grouped.length - 1];
      if (last?.date === txn.date) last.transactions.push(txn);
      else grouped.push({ date: txn.date, transactions: [txn] });
    }
    return grouped;
  }, [activity, filter, today]);

  return (
    <Card style={styles.card}>
      <CardTitle
        title="Recent activity"
        right={
          <Pressable style={styles.inline} onPress={onSeeAll} hitSlop={8}>
            <AppText variant="caption" weight="semibold" tone="primary">
              See all
            </AppText>
            <ArrowRight size={13} color={colors.primary} />
          </Pressable>
        }
      />
      <View style={styles.cardHead}>
        <Segmented options={FILTERS} value={filter} onChange={setFilter} />
        {needsReview > 0 ? (
          <Pressable style={styles.reviewPill} onPress={onSeeAll}>
            <Sparkles size={12} color={colors.primary} />
            <AppText variant="caption" weight="semibold" tone="primary">
              {needsReview} to review
            </AppText>
          </Pressable>
        ) : null}
      </View>
      {days.length === 0 ? (
        <AppText variant="caption" tone="faint">
          {filter === 'all' ? 'No transactions yet.' : `No ${filter === 'in' ? 'income' : 'spending'} lately.`}
        </AppText>
      ) : (
        days.map((day) => (
          <View key={day.date} style={styles.day}>
            <AppText variant="label" tone="faint">
              {formatRelativeDay(day.date, today)}
            </AppText>
            {day.transactions.map((txn) => {
              const net = (txn.inflow ?? 0) - (txn.outflow ?? 0);
              const transfer = Boolean(txn.transferAccountId);
              const income = !transfer && inflowCategoryId && txn.categoryId === inflowCategoryId;
              const other = transfer ? accounts.find((account) => account.id === txn.transferAccountId)?.name : null;
              const title = transfer ? `${net < 0 ? 'To' : 'From'} ${other ?? 'another account'}` : txn.payeeName || 'Unknown payee';
              const detail = transfer ? 'Transfer' : income ? 'Ready to assign' : txn.categoryName || 'Uncategorized';
              return (
                <View key={txn.id} style={styles.activityRow}>
                  {transfer ? (
                    <View style={styles.roundIcon}>
                      <ArrowLeftRight size={16} color={colors.muted} />
                    </View>
                  ) : income ? (
                    <View style={[styles.roundIcon, { backgroundColor: colors.successMuted }]}>
                      <ArrowDownLeft size={16} color={colors.success} />
                    </View>
                  ) : (
                    <InitialAvatar name={title} size={36} />
                  )}
                  <View style={styles.grow}>
                    <AppText weight="medium" numberOfLines={1}>
                      {title}
                    </AppText>
                    <AppText variant="caption" muted numberOfLines={1}>
                      <AppText variant="caption" style={!transfer && !txn.categoryId ? styles.warnText : undefined}>
                        {detail}
                      </AppText>
                      {` · ${txn.accountName}`}
                    </AppText>
                  </View>
                  <View style={styles.activityEnd}>
                    <AppText weight="semibold" tabular tone={net > 0 && !transfer ? 'success' : undefined}>
                      {formatSignedCurrency(net, !transfer)}
                    </AppText>
                    {txn.status === TransactionStatus.UNAPPROVED ? (
                      <AppText variant="caption" tone="primary">
                        Review
                      </AppText>
                    ) : null}
                  </View>
                </View>
              );
            })}
          </View>
        ))
      )}
    </Card>
  );
}

// ── Screen ──

export function DashboardScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation<Nav>();
  const user = useAppSelector((state) => state.auth.user);
  const budgetId = useAppSelector((state) => state.budgets.selectedBudget?.id);
  const month = useAppSelector((state) => state.budgets.selectedMonth);
  const revision = useAppSelector((state) => state.dashboard.revision);
  const error = useAppSelector((state) => state.dashboard.error);
  const inflowAmount = useAppSelector((state) => state.categories.inflowAmount);
  const inflowSuccess = useAppSelector((state) => state.categories.inflowLoading === 'success');
  // keep the pill up through refetches; only the very first load hides it
  const [inflowLoaded, setInflowLoaded] = useState(inflowSuccess);
  if (!inflowLoaded && inflowSuccess) setInflowLoaded(true);
  const loanAccounts = useAppSelector((state) => state.accounts.loanAccounts);
  const refreshing = useAppSelector(
    (state) => state.dashboard.activityLoading === 'pending' || state.dashboard.cashFlowLoading === 'pending'
  );

  // balance history walks back from the current balances through the
  // activity, so both reload together whenever a transaction changes
  useEffect(() => {
    if (!budgetId) return;
    dispatch(fetchDashboardActivity());
  }, [dispatch, budgetId, revision]);

  useEffect(() => {
    if (!budgetId || !month) return;
    dispatch(fetchDashboardCashFlow(month));
  }, [dispatch, budgetId, month, revision]);

  const refresh = () => {
    dispatch(fetchAllAccounts());
    dispatch(fetchInflowAmount());
    if (month) dispatch(fetchAllCategoryGroups(month));
    dispatch(fetchDashboardActivity());
    if (month) dispatch(fetchDashboardCashFlow(month));
  };

  const goTransactions = () => navigation.navigate('Main', { screen: 'Transactions' });
  const firstName = user?.name?.trim().split(/\s+/)[0];

  return (
    <Screen
      style={styles.screen}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={refresh} tintColor={colors.primary} />}
    >
      <View style={styles.headerRow}>
        <View style={styles.grow}>
          <AppText variant="caption" muted>
            {new Intl.DateTimeFormat('en-IN', { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date())}
          </AppText>
          <AppText variant="title">{`${greeting()}${firstName ? `, ${firstName}` : ''}`}</AppText>
        </View>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Open settings"
          style={({ pressed }) => [styles.avatarButton, pressed && styles.pressed]}
          onPress={() => navigation.navigate('Settings')}
        >
          {user?.picture ? (
            <Image source={{ uri: user.picture }} style={styles.avatar} />
          ) : (
            <UserRound size={20} color={colors.primary} />
          )}
        </Pressable>
      </View>

      {inflowLoaded ? (
        <Pressable
          style={[styles.ready, inflowAmount < 0 && styles.readyOver, inflowAmount === 0 && styles.readyDone]}
          onPress={() => navigation.navigate('Main', { screen: 'Budget' })}
        >
          {inflowAmount < 0 ? (
            <TriangleAlert size={15} color={colors.danger} />
          ) : inflowAmount === 0 ? (
            <CheckCircle2 size={15} color={colors.success} />
          ) : null}
          <AppText variant="caption" weight="semibold" style={styles.grow}>
            {inflowAmount < 0
              ? `Assigned ${formatCurrency(Math.abs(inflowAmount))} more than you have`
              : inflowAmount === 0
                ? 'Every rupee has a job'
                : `${formatCurrency(inflowAmount)} ready to assign`}
          </AppText>
          <ArrowRight size={14} color={colors.muted} />
        </Pressable>
      ) : null}

      {error ? (
        <Pressable style={styles.errorBox} onPress={refresh}>
          <AlertCircle size={16} color={colors.danger} />
          <AppText variant="caption" style={styles.grow}>
            Some numbers didn't load. Tap to retry.
          </AppText>
        </Pressable>
      ) : null}

      <BalanceCard />
      <CashFlowCard />
      <SpendingCard onUncategorized={goTransactions} />
      <RecentActivity onSeeAll={goTransactions} />

      {loanAccounts.length ? (
        <Pressable style={styles.loansRow} onPress={() => navigation.navigate('Loans')}>
          <View style={[styles.roundIcon, { backgroundColor: colors.primaryMuted }]}>
            <Landmark size={16} color={colors.primary} />
          </View>
          <View style={styles.grow}>
            <AppText weight="medium">Loans</AppText>
            <AppText variant="caption" muted>
              {loanAccounts.length === 1 ? loanAccounts[0].name : `${loanAccounts.length} loans`} · payoff progress
            </AppText>
          </View>
          <ArrowRight size={16} color={colors.muted} />
        </Pressable>
      ) : null}

      <RecentDocumentsCard />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  grow: {
    flex: 1,
    minWidth: 0
  },
  center: {
    textAlign: 'center'
  },
  pressed: {
    opacity: 0.7
  },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  avatarButton: {
    width: 42,
    height: 42,
    borderRadius: 21,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryMuted,
    overflow: 'hidden'
  },
  avatar: {
    width: 42,
    height: 42,
    borderRadius: 21
  },
  ready: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderRadius: radii.full,
    backgroundColor: colors.primaryMuted,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md
  },
  readyOver: {
    backgroundColor: colors.dangerMuted
  },
  readyDone: {
    backgroundColor: colors.successMuted
  },
  errorBox: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderRadius: radii.md,
    backgroundColor: colors.dangerMuted,
    padding: spacing.md
  },
  card: {
    gap: spacing.md
  },
  cardHead: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  inline: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4
  },
  linkRow: {
    alignSelf: 'flex-start'
  },
  accountRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingVertical: spacing.xs
  },
  check: {
    width: 20,
    height: 20,
    borderRadius: 6,
    borderWidth: 1.5,
    borderColor: colors.faint,
    alignItems: 'center',
    justifyContent: 'center'
  },
  checkOn: {
    backgroundColor: colors.primary,
    borderColor: colors.primary
  },
  flowRow: {
    flexDirection: 'row',
    gap: spacing.lg
  },
  flowCell: {
    flex: 1,
    gap: spacing.xs
  },
  flowIcon: {
    width: 24,
    height: 24,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 4
  },
  netBox: {
    gap: spacing.sm,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    padding: spacing.md
  },
  meter: {
    height: 8,
    borderRadius: 4,
    backgroundColor: colors.surfaceTertiary,
    overflow: 'hidden'
  },
  meterFill: {
    height: '100%',
    borderRadius: 4,
    backgroundColor: colors.primary
  },
  meterOver: {
    backgroundColor: colors.danger
  },
  groupRow: {
    gap: 6,
    borderRadius: radii.md,
    padding: spacing.sm
  },
  groupRowActive: {
    backgroundColor: colors.surfaceStrong
  },
  bar: {
    height: 5,
    borderRadius: 3,
    backgroundColor: colors.surfaceStrong,
    overflow: 'hidden'
  },
  barFill: {
    height: '100%',
    borderRadius: 3,
    backgroundColor: chartSeries[0]
  },
  tilesHead: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    marginTop: spacing.sm
  },
  tiles: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    rowGap: spacing.lg
  },
  tile: {
    width: '33.33%',
    alignItems: 'center',
    gap: 2,
    paddingHorizontal: 2
  },
  ringText: {
    fontSize: 12
  },
  tileStatus: {
    fontSize: 11
  },
  attention: {
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
  reviewPill: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    borderRadius: radii.full,
    backgroundColor: colors.primaryMuted,
    paddingHorizontal: spacing.md,
    paddingVertical: 6
  },
  day: {
    gap: spacing.sm
  },
  activityRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  activityEnd: {
    alignItems: 'flex-end'
  },
  roundIcon: {
    width: 36,
    height: 36,
    borderRadius: 18,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  loansRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    borderRadius: radii.lg,
    backgroundColor: colors.surface,
    padding: spacing.lg
  }
});
