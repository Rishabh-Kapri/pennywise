import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Image, Pressable, RefreshControl, ScrollView, StyleSheet, View } from 'react-native';
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
  ChevronRight,
  Landmark,
  Sparkles,
  TriangleAlert,
  UserRound
} from 'lucide-react-native';
import type { RootStackParamList } from '../../../navigation/types';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { CircleButton, LetterTile, PillBars, SectionHead } from '../../../components/Blocks';
import { Hatch, HatchMeter } from '../../../components/Hatch';
import { ProgressBar } from '../../../components/ProgressBar';
import { LineChart, Segmented } from '../../../components/charts';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { Screen } from '../../../components/Screen';
import { chartSeries, colors, radii, screenPadding, spacing } from '../../../theme';
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
import { CardWalletRow } from '../../accounts/components/CardWallet';
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
  selectMonthlySpending,
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

function CardTitle({ title, subtitle, right }: { title: string; subtitle?: string; right?: ReactNode }) {
  return (
    <View style={styles.cardHead}>
      <View style={styles.grow}>
        <AppText variant="heading">{title}</AppText>
        {subtitle ? (
          <AppText variant="caption" muted numberOfLines={1}>
            {subtitle}
          </AppText>
        ) : null}
      </View>
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

/** Ready-to-assign state as a chip on the hero card. */
function ReadyChip({ onPress }: { onPress: () => void }) {
  const inflowAmount = useAppSelector((state) => state.categories.inflowAmount);
  const inflowSuccess = useAppSelector((state) => state.categories.inflowLoading === 'success');
  // keep the chip up through refetches; only the very first load hides it
  const [loaded, setLoaded] = useState(inflowSuccess);
  if (!loaded && inflowSuccess) setLoaded(true);
  if (!loaded) return null;

  const over = inflowAmount < 0;
  const done = inflowAmount === 0;
  return (
    <Pressable style={[styles.readyChip, over && styles.readyChipOver]} onPress={onPress} accessibilityRole="button">
      {over ? <TriangleAlert size={12} color={colors.danger} /> : done ? <CheckCircle2 size={12} color={colors.success} /> : null}
      <AppText variant="caption" weight="semibold" style={styles.onHero} numberOfLines={1}>
        {over
          ? `Assigned ${formatCurrency(Math.abs(inflowAmount))} too much`
          : done
            ? 'Every rupee has a job'
            : `${formatCurrency(inflowAmount)} ready to assign`}
      </AppText>
      <ChevronRight size={12} color={colors.onHeroMuted} />
    </Pressable>
  );
}

function useBalance() {
  const accounts = useAppSelector(selectDashboardAccounts);
  const activity = useAppSelector((state) => state.dashboard.activity);
  const activityWindow = useAppSelector(selectActivityWindow);
  const budgetId = useAppSelector((state) => state.budgets.selectedBudget?.id);
  const [excluded, toggle] = useExcludedAccounts(budgetId);
  const [range, setRange] = useState<Range>('3m');
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

  return { accounts, ids, total, history, range, setRange, toggle };
}

type Balance = ReturnType<typeof useBalance>;

function BalanceHero({ balance, onReady }: { balance: Balance; onReady: () => void }) {
  const { accounts, ids, total, history } = balance;
  const first = history[0];
  const change = first ? total - first.balance : 0;

  return (
    <View style={styles.hero}>
      <Hatch color={colors.onHero} gap={9} opacity={0.05} />
      <ReadyChip onPress={onReady} />
      <AppText variant="caption" style={[styles.onHeroMuted, styles.heroLabel]}>
        {ids.size === accounts.length
          ? `Balance across ${accounts.length === 1 ? 'your account' : `all ${accounts.length} accounts`}`
          : `Balance across ${ids.size} of ${accounts.length} accounts`}
      </AppText>
      <AppText variant="display" tabular style={[styles.onHero, styles.heroAmount]} numberOfLines={1} adjustsFontSizeToFit>
        {formatSignedCurrency(total)}
      </AppText>
      {first && history.length > 1 ? (
        <View style={[styles.inline, styles.centerRow]}>
          {Math.round(change) === 0 ? (
            <AppText variant="caption" style={styles.onHeroMuted}>
              No change
            </AppText>
          ) : (
            <>
              {change > 0 ? <ArrowUp size={13} color={colors.heroPositive} /> : <ArrowDown size={13} color={colors.heroNegative} />}
              <AppText variant="caption" weight="semibold" tabular style={{ color: change > 0 ? colors.heroPositive : colors.heroNegative }}>
                {formatSignedCurrency(change, true)}
              </AppText>
            </>
          )}
          <AppText variant="caption" style={styles.onHeroMuted}>
            {' '}
            since {formatDayMonth(first.date)}
          </AppText>
        </View>
      ) : null}
    </View>
  );
}

function BalanceTrendCard({ balance }: { balance: Balance }) {
  const { accounts, ids, history, range, setRange, toggle } = balance;
  const [showAccounts, setShowAccounts] = useState(false);
  const [selected, setSelected] = useState<number | null>(null);

  // a point per day is too fine for a phone-width line; sample down to ~45
  const points = useMemo(() => {
    const step = Math.max(1, Math.ceil(history.length / 45));
    return history
      .filter((_, i) => i % step === 0 || i === history.length - 1)
      .map((point) => ({ key: point.date, label: formatDayMonth(point.date), values: { balance: point.balance } }));
  }, [history]);
  const pick = selected != null ? points[selected] : null;

  return (
    <Card style={styles.card}>
      <CardTitle
        title="Balance trend"
        subtitle={pick ? `${pick.label} · ${formatSignedCurrency(pick.values.balance)}` : 'Tap the line for a day'}
        right={<Segmented options={RANGES} value={range} onChange={(value) => { setRange(value); setSelected(null); }} />}
      />
      <View style={styles.inset}>
        {points.length > 1 ? (
          <LineChart
            points={points}
            series={[{ key: 'balance', label: 'Balance', color: chartSeries[0] }]}
            height={140}
            selected={selected}
            onSelect={(index) => setSelected((current) => (current === index ? null : index))}
          />
        ) : (
          <AppText variant="caption" tone="faint">
            {ids.size === 0 ? 'Tick an account to chart its balance.' : 'Not enough history yet.'}
          </AppText>
        )}
      </View>

      <Pressable style={styles.linkRowBox} onPress={() => setShowAccounts((open) => !open)} accessibilityRole="button">
        <AppText weight="medium" style={styles.grow}>
          {showAccounts ? 'Hide accounts' : `Choose accounts · ${ids.size} of ${accounts.length}`}
        </AppText>
        <ArrowRight size={18} color={colors.text} style={showAccounts ? styles.rotateDown : undefined} />
      </Pressable>
      {showAccounts
        ? accounts.map((account) => {
            const included = Boolean(account.id && ids.has(account.id));
            const accountBalance = account.balance ?? 0;
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
                <AppText weight="semibold" tabular tone={accountBalance < 0 ? 'danger' : undefined}>
                  {formatSignedCurrency(accountBalance)}
                </AppText>
              </Pressable>
            );
          })
        : null}
    </Card>
  );
}

// ── This month ──

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

function ThisMonthCard({ onReports }: { onReports: () => void }) {
  const flow = useAppSelector(selectMonthCashFlow);
  if (!flow) return null;
  const prevName = formatMonthName(shiftMonth(flow.month, -1), 'short');
  const spentShare = flow.income > 0 ? flow.spending / flow.income : null;
  const overspent = flow.net < 0;

  return (
    <Card style={styles.card}>
      <View style={styles.cardHead}>
        <View style={styles.grow}>
          <AppText variant="caption" muted>
            Spent in {formatMonthName(flow.month)}
          </AppText>
          <AppText variant="title" tabular numberOfLines={1} adjustsFontSizeToFit>
            {formatCurrency(flow.spending)}
          </AppText>
          <Change current={flow.spending} previous={flow.prevSpending} upIsGood={false} against={prevName} />
        </View>
        <CircleButton label="Open reports" onPress={onReports}>
          <ArrowUpRight size={18} color={colors.text} />
        </CircleButton>
      </View>

      <View style={styles.tilePair}>
        <View style={styles.statTile}>
          <AppText variant="caption" muted>
            Income
          </AppText>
          <AppText weight="bold" tabular tone="success" numberOfLines={1} adjustsFontSizeToFit style={styles.statValue}>
            {formatSignedCurrency(flow.income, true)}
          </AppText>
          <Change current={flow.income} previous={flow.prevIncome} upIsGood against={prevName} />
        </View>
        <View style={styles.statTile}>
          <AppText variant="caption" muted>
            {overspent ? 'Overspent' : 'Left over'}
          </AppText>
          <AppText weight="bold" tabular tone={overspent ? 'danger' : 'default'} numberOfLines={1} adjustsFontSizeToFit style={styles.statValue}>
            {formatCurrency(Math.abs(flow.net))}
          </AppText>
          <AppText variant="caption" tone="faint">
            {spentShare === null ? 'No income yet' : `${Math.round(spentShare * 100)}% of income spent`}
          </AppText>
        </View>
      </View>

      <View style={styles.box}>
        <HatchMeter percent={spentShare === null ? 0 : spentShare * 100} />
        <View style={styles.cardHead}>
          <AppText variant="caption" muted style={styles.grow}>
            Of income
          </AppText>
          <AppText variant="caption" muted tabular>
            <AppText variant="caption" weight="semibold">
              {formatCurrency(flow.spending)}
            </AppText>
            {` / ${formatCurrency(flow.income)}`}
          </AppText>
        </View>
      </View>

      {flow.avgMonths > 0 ? (
        <AppText variant="caption" tone="faint">
          {`Monthly average, last ${flow.avgMonths === 1 ? 'month' : `${flow.avgMonths} months`}: +${formatCurrency(flow.avgIncome)} in · −${formatCurrency(flow.avgSpending)} out`}
        </AppText>
      ) : null}
    </Card>
  );
}

// ── Spending by month ──

function SpendingBarsCard() {
  const months = useAppSelector(selectMonthlySpending);
  if (months.length < 2) return null;
  return (
    <Card>
      <PillBars title="Spending analytics" points={months.map((m) => ({ month: m.month, value: m.spending }))} />
    </Card>
  );
}

// ── Categories ──

function CategoryGoal({ name, spent, budgeted, color, onPress }: { name: string; spent: number; budgeted: number; color: string; onPress: () => void }) {
  const hasBudget = budgeted > 0;
  const over = hasBudget && spent > budgeted;
  const percent = hasBudget ? (spent / budgeted) * 100 : 0;
  return (
    <Pressable style={({ pressed }) => [styles.goalCard, pressed && styles.pressed]} onPress={onPress} accessibilityRole="button">
      <View style={styles.cardHead}>
        <LetterTile name={name} color={color} />
        <AppText weight="semibold" numberOfLines={1} style={styles.grow}>
          {name}
        </AppText>
        <CircleButton label={`Open ${name}`}>
          <ChevronRight size={16} color={colors.text} />
        </CircleButton>
      </View>
      <View style={styles.goalBox}>
        <View style={styles.cardHead}>
          <AppText weight="bold" tabular style={styles.grow}>
            {formatCurrency(spent)}
          </AppText>
          <AppText variant="caption" tone="faint" tabular>
            {hasBudget ? formatCurrency(budgeted) : 'Not budgeted'}
          </AppText>
        </View>
        <ProgressBar percent={percent} height={4} color={over ? colors.danger : color} />
        <View style={styles.cardHead}>
          <AppText variant="caption" tone={over ? 'danger' : 'muted'} numberOfLines={1} style={styles.grow}>
            {over
              ? `${formatCurrency(spent - budgeted)} over`
              : !hasBudget
                ? 'Nothing assigned'
                : spent === budgeted
                  ? 'Fully spent'
                  : `${formatCurrency(budgeted - spent)} left`}
          </AppText>
          {hasBudget ? (
            <AppText variant="caption" weight="bold" tabular>
              {`${Math.round(percent)}%`}
            </AppText>
          ) : null}
        </View>
      </View>
    </Pressable>
  );
}

const MAX_CATEGORIES = 5;

function CategoriesSection({
  onBudget,
  onUncategorized,
  onCategory
}: {
  onBudget: () => void;
  onUncategorized: () => void;
  onCategory: (id: string) => void;
}) {
  const summary = useAppSelector(selectSpendingSummary);
  const activity = useAppSelector((state) => state.dashboard.activity);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  if (!summary) return null;
  const month = summary.month;
  const uncategorized = activity.filter(
    (txn) => txn.date.startsWith(month) && !txn.categoryId && !txn.transferAccountId
  ).length;
  const groups = summary.groups;
  const selectedIndex = Math.max(0, groups.findIndex((group) => group.id === selectedId));
  const selected: SpendingGroup | undefined = groups[selectedIndex];
  const color = chartSeries[selectedIndex % chartSeries.length];
  const categories = selected ? [...selected.categories].sort((a, b) => b.spent - a.spent).slice(0, MAX_CATEGORIES) : [];

  return (
    <View style={styles.section}>
      <SectionHead title="Your categories" onViewAll={onBudget} />
      {!selected ? (
        <Card>
          <AppText variant="caption" tone="faint">
            No spending in {formatMonthName(month)} yet.
          </AppText>
        </Card>
      ) : (
        <>
          <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.chipsBleed} contentContainerStyle={styles.chips}>
            {groups.map((group) => {
              const active = group.id === selected.id;
              return (
                <Pressable
                  key={group.id}
                  style={[styles.chip, active && styles.chipActive]}
                  onPress={() => setSelectedId(group.id)}
                  accessibilityRole="button"
                  accessibilityState={{ selected: active }}
                >
                  <AppText variant="caption" weight="semibold" style={active ? styles.chipActiveText : undefined} numberOfLines={1}>
                    {group.name}
                  </AppText>
                  <AppText variant="caption" tabular style={active ? styles.chipActiveSub : styles.chipSub}>
                    {formatCompactCurrency(group.spent)}
                  </AppText>
                </Pressable>
              );
            })}
          </ScrollView>
          {categories.map((category) => (
            <CategoryGoal
              key={category.id}
              name={category.name}
              spent={category.spent}
              budgeted={category.budgeted}
              color={color}
              onPress={() => onCategory(category.id)}
            />
          ))}
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
    </View>
  );
}

// ── Recent activity ──

const FILTERS = [
  { value: 'all', label: 'All' },
  { value: 'out', label: 'Spending' },
  { value: 'in', label: 'Income' }
] as const;
type Filter = (typeof FILTERS)[number]['value'];

const RECENT_LIMIT = 8;

function RecentActivity({ onSeeAll }: { onSeeAll: () => void }) {
  const activity = useAppSelector((state) => state.dashboard.activity);
  const needsReview = useAppSelector(selectNeedsReviewCount);
  const inflowCategoryId = useAppSelector((state) => state.budgets.selectedBudget?.metadata?.inflowCategoryId);
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const [filter, setFilter] = useState<Filter>('all');
  const today = todayKey();

  const recent = useMemo(() => {
    const matches = (txn: Transaction) => {
      if (filter === 'all') return true;
      if (txn.transferAccountId) return false;
      return filter === 'in' ? (txn.inflow ?? 0) > 0 : (txn.outflow ?? 0) > 0;
    };
    return activity.filter((t) => t.date <= today && matches(t)).slice(0, RECENT_LIMIT);
  }, [activity, filter, today]);

  return (
    <View style={styles.section}>
      <SectionHead title="Recent transactions" onViewAll={onSeeAll} />
      <Card style={styles.card}>
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
        {recent.length === 0 ? (
          <AppText variant="caption" tone="faint">
            {filter === 'all' ? 'No transactions yet.' : `No ${filter === 'in' ? 'income' : 'spending'} lately.`}
          </AppText>
        ) : (
          recent.map((txn) => {
            const net = (txn.inflow ?? 0) - (txn.outflow ?? 0);
            const transfer = Boolean(txn.transferAccountId);
            const income = !transfer && inflowCategoryId && txn.categoryId === inflowCategoryId;
            const other = transfer ? accounts.find((account) => account.id === txn.transferAccountId)?.name : null;
            const title = transfer ? `${net < 0 ? 'To' : 'From'} ${other ?? 'another account'}` : txn.payeeName || 'Unknown payee';
            const detail = transfer ? 'Transfer' : income ? 'Ready to assign' : txn.categoryName || 'Uncategorized';
            const review = txn.status === TransactionStatus.UNAPPROVED;
            return (
              <View key={txn.id} style={styles.txnCard}>
                <View style={styles.txnMain}>
                  <View style={styles.txnTile}>
                    {transfer ? (
                      <ArrowLeftRight size={18} color={colors.muted} />
                    ) : income ? (
                      <ArrowDownLeft size={18} color={colors.success} />
                    ) : (
                      <InitialAvatar name={title} size={34} />
                    )}
                  </View>
                  <View style={styles.grow}>
                    <AppText weight="semibold" numberOfLines={1}>
                      {title}
                    </AppText>
                    <AppText variant="caption" muted numberOfLines={1}>
                      {txn.accountName}
                    </AppText>
                  </View>
                  <View style={styles.endText}>
                    <AppText weight="bold" tabular tone={net > 0 && !transfer ? 'success' : undefined}>
                      {formatSignedCurrency(net, !transfer)}
                    </AppText>
                    <AppText variant="caption" tone="faint">
                      {formatRelativeDay(txn.date, today)}
                    </AppText>
                  </View>
                </View>
                <View style={styles.txnFoot}>
                  <AppText
                    variant="caption"
                    muted
                    numberOfLines={1}
                    style={[styles.grow, !transfer && !txn.categoryId ? styles.warnText : undefined]}
                  >
                    {detail}
                  </AppText>
                  <AppText variant="caption" weight="semibold" tone={review ? 'primary' : 'success'}>
                    {review ? 'Needs review' : 'Approved'}
                  </AppText>
                </View>
              </View>
            );
          })
        )}
      </Card>
    </View>
  );
}

// ── Screen ──

export function DashboardScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation<Nav>();
  const user = useAppSelector((state) => state.auth.user);
  const budgetId = useAppSelector((state) => state.budgets.selectedBudget?.id);
  const budgetName = useAppSelector((state) => state.budgets.selectedBudget?.name);
  const month = useAppSelector((state) => state.budgets.selectedMonth);
  const revision = useAppSelector((state) => state.dashboard.revision);
  const error = useAppSelector((state) => state.dashboard.error);
  const loanAccounts = useAppSelector((state) => state.accounts.loanAccounts);
  const refreshing = useAppSelector(
    (state) => state.dashboard.activityLoading === 'pending' || state.dashboard.cashFlowLoading === 'pending'
  );
  const balance = useBalance();

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
  const goBudget = () => navigation.navigate('Main', { screen: 'Budget' });
  const goReports = () => navigation.navigate('Main', { screen: 'Reports' });
  // the category screen reads the budget's selected month, which is also the
  // month the dashboard's spending report ends on
  const goCategory = (categoryId: string) => navigation.navigate('Category', { id: categoryId });
  const firstName = user?.name?.trim().split(/\s+/)[0];

  return (
    <Screen
      style={styles.screen}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={refresh} tintColor={colors.primary} />}
    >
      <View style={styles.headerRow}>
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
        <View style={styles.grow}>
          <AppText variant="heading" numberOfLines={1}>{`${greeting()}${firstName ? `, ${firstName}` : ''}`}</AppText>
          <AppText variant="caption" muted numberOfLines={1}>
            {[budgetName, new Intl.DateTimeFormat('en-IN', { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date())]
              .filter(Boolean)
              .join(' · ')}
          </AppText>
        </View>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Ask Penny"
          style={({ pressed }) => [styles.headerButton, pressed && styles.pressed]}
          onPress={() => navigation.navigate('Penny')}
        >
          <Sparkles size={18} color={colors.text} />
        </Pressable>
      </View>

      {error ? (
        <Pressable style={styles.errorBox} onPress={refresh}>
          <AlertCircle size={16} color={colors.danger} />
          <AppText variant="caption" style={styles.grow}>
            Some numbers didn't load. Tap to retry.
          </AppText>
        </Pressable>
      ) : null}

      <BalanceHero balance={balance} onReady={goBudget} />
      <CardWalletRow />
      <ThisMonthCard onReports={goReports} />
      <SpendingBarsCard />
      <CategoriesSection onBudget={goBudget} onUncategorized={goTransactions} onCategory={goCategory} />
      <RecentActivity onSeeAll={goTransactions} />
      <BalanceTrendCard balance={balance} />

      {loanAccounts.length ? (
        <Pressable style={styles.goalCard} onPress={() => navigation.navigate('Loans')}>
          <View style={styles.cardHead}>
            <View style={styles.loanTile}>
              <Landmark size={18} color={colors.primary} />
            </View>
            <View style={styles.grow}>
              <AppText weight="semibold">Loans</AppText>
              <AppText variant="caption" muted>
                {loanAccounts.length === 1 ? loanAccounts[0].name : `${loanAccounts.length} loans`} · payoff progress
              </AppText>
            </View>
            <CircleButton label="Open loans">
              <ChevronRight size={16} color={colors.text} />
            </CircleButton>
          </View>
        </Pressable>
      ) : null}

      <RecentDocumentsCard />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.md
  },
  section: {
    gap: spacing.sm
  },
  grow: {
    flex: 1,
    minWidth: 0
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
    width: 44,
    height: 44,
    borderRadius: 22,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryMuted,
    overflow: 'hidden'
  },
  avatar: {
    width: 44,
    height: 44,
    borderRadius: 22
  },
  headerButton: {
    width: 44,
    height: 44,
    borderRadius: 22,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surface
  },
  hero: {
    alignItems: 'center',
    gap: spacing.xs,
    borderRadius: radii.xl,
    backgroundColor: colors.hero,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.heroBorder,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.lg,
    overflow: 'hidden'
  },
  heroLabel: {
    marginTop: spacing.sm
  },
  heroAmount: {
    textAlign: 'center'
  },
  centerRow: {
    justifyContent: 'center'
  },
  onHero: {
    color: colors.onHero
  },
  onHeroMuted: {
    color: colors.onHeroMuted
  },
  readyChip: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    maxWidth: '100%',
    borderRadius: radii.full,
    backgroundColor: colors.heroControl,
    paddingHorizontal: spacing.md,
    paddingVertical: 6
  },
  readyChipOver: {
    backgroundColor: 'rgba(239, 107, 107, 0.22)'
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
    gap: 10
  },
  cardHead: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  inline: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4
  },
  endText: {
    alignItems: 'flex-end',
    maxWidth: '45%'
  },
  // a bordered panel inside a card
  box: {
    gap: spacing.sm,
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: colors.border,
    padding: 10
  },
  // a grey panel inside a card (charts)
  inset: {
    gap: spacing.sm,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    padding: 10
  },
  tilePair: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  statTile: {
    flex: 1,
    alignItems: 'center',
    gap: 2,
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: colors.border,
    paddingVertical: spacing.sm,
    paddingHorizontal: spacing.sm
  },
  statValue: {
    fontSize: 17
  },
  linkRowBox: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: colors.border,
    paddingHorizontal: spacing.md,
    paddingVertical: 10
  },
  rotateDown: {
    transform: [{ rotate: '90deg' }]
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
  chipsBleed: {
    marginHorizontal: -screenPadding
  },
  chips: {
    paddingHorizontal: screenPadding,
    gap: spacing.sm
  },
  chip: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    borderRadius: radii.full,
    backgroundColor: colors.surface,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm
  },
  chipActive: {
    backgroundColor: colors.text
  },
  chipActiveText: {
    color: colors.background
  },
  chipSub: {
    color: colors.faint
  },
  chipActiveSub: {
    color: colors.background,
    opacity: 0.7
  },
  goalCard: {
    gap: spacing.sm,
    borderRadius: radii.lg,
    backgroundColor: colors.surface,
    padding: 10
  },
  goalBox: {
    gap: 6,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    paddingHorizontal: spacing.md,
    paddingVertical: 10
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
  txnCard: {
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: colors.border,
    overflow: 'hidden'
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
    borderRadius: 13,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  loanTile: {
    width: 40,
    height: 40,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface
  },
  txnFoot: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    backgroundColor: colors.surfaceStrong,
    paddingHorizontal: 10,
    paddingVertical: 6
  }
});
