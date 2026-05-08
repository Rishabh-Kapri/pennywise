import { RefreshControl, StyleSheet, View } from 'react-native';
import { ArrowDownLeft, ArrowUpRight, Landmark, PieChart, WalletCards } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { SectionHeader } from '../../../components/SectionHeader';
import { fetchAllAccounts } from '../../accounts/store/accountSlice';
import { fetchAllCategoryGroups, fetchInflowAmount } from '../../category/store/categorySlice';
import { fetchAllTransactions } from '../../transactions/store/transactionSlice';
import { selectMonthInHumanFormat, selectSelectedMonth } from '../../budget/store/budgetSlice';
import { formatCurrency, formatShortDate } from '../../../utils/date';
import { colors, spacing } from '../../../theme';

function greeting() {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 17) return 'Good afternoon';
  return 'Good evening';
}

export function DashboardScreen() {
  const dispatch = useAppDispatch();
  const selectedMonth = useAppSelector(selectSelectedMonth);
  const monthLabel = useAppSelector(selectMonthInHumanFormat);
  const accounts = useAppSelector((state) => [...state.accounts.budgetAccounts, ...state.accounts.trackingAccounts]);
  const transactions = useAppSelector((state) => state.transactions.transactions);
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  const user = useAppSelector((state) => state.auth.user);
  const refreshing = useAppSelector((state) => state.accounts.loading === 'pending' || state.transactions.loading === 'pending');

  const totalBalance = accounts.reduce((sum, account) => sum + (account.balance ?? 0), 0);
  const availableCash = accounts.filter((account) => (account.balance ?? 0) > 0).reduce((sum, account) => sum + (account.balance ?? 0), 0);
  const debt = Math.abs(accounts.filter((account) => (account.balance ?? 0) < 0).reduce((sum, account) => sum + (account.balance ?? 0), 0));
  const totalInflow = transactions.reduce((sum, txn) => sum + (txn.inflow ?? 0), 0);
  const totalOutflow = transactions.reduce((sum, txn) => sum + (txn.outflow ?? 0), 0);

  const categories = groups.flatMap((group) =>
    group.categories.map((category) => ({
      id: category.id ?? category.name,
      name: category.name,
      budgeted: category.budgeted?.[selectedMonth] ?? 0,
      spent: Math.abs(category.activity?.[selectedMonth] ?? 0),
      remaining: category.balance?.[selectedMonth] ?? 0
    }))
  );
  const overspent = categories.filter((category) => category.remaining < 0).sort((a, b) => a.remaining - b.remaining).slice(0, 3);
  const topSpent = categories.filter((category) => category.spent > 0).sort((a, b) => b.spent - a.spent).slice(0, 4);

  const refresh = () => {
    dispatch(fetchAllAccounts());
    dispatch(fetchAllTransactions());
    if (selectedMonth) dispatch(fetchAllCategoryGroups(selectedMonth));
    dispatch(fetchInflowAmount());
  };

  return (
    <Screen
      style={styles.screen}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={refresh} tintColor={colors.primary} />}
    >
      <SectionHeader
        title={`${greeting()}${user?.name ? `, ${user.name.split(' ')[0]}` : ''}`}
        subtitle={new Intl.DateTimeFormat('en-IN', { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date())}
      />

      <View style={styles.statGrid}>
        <Card style={styles.statCard}>
          <WalletCards size={18} color={colors.primary} />
          <AppText muted>Total balance</AppText>
          <AppText weight="bold" style={styles.statValue}>{formatCurrency(totalBalance)}</AppText>
        </Card>
        <Card style={styles.statCard}>
          <Landmark size={18} color={colors.primary} />
          <AppText muted>Available cash</AppText>
          <AppText weight="bold" style={styles.statValue}>{formatCurrency(availableCash)}</AppText>
        </Card>
        <Card style={styles.statCard}>
          <ArrowDownLeft size={18} color={colors.danger} />
          <AppText muted>Debt</AppText>
          <AppText weight="bold" style={styles.statValue}>{formatCurrency(debt)}</AppText>
        </Card>
        <Card style={styles.statCard}>
          <PieChart size={18} color={colors.primary} />
          <AppText muted>Accounts</AppText>
          <AppText weight="bold" style={styles.statValue}>{accounts.length}</AppText>
        </Card>
      </View>

      <Card style={styles.cardGap}>
        <View style={styles.rowBetween}>
          <AppText weight="semibold" style={styles.cardTitle}>Budget Overview</AppText>
          <AppText muted>{monthLabel || 'This month'}</AppText>
        </View>
        <View style={styles.moneyRow}>
          <View>
            <AppText muted>Incoming</AppText>
            <AppText weight="bold" style={styles.positive}>+{formatCurrency(totalInflow)}</AppText>
          </View>
          <View>
            <AppText muted>Outgoing</AppText>
            <AppText weight="bold" style={styles.negative}>-{formatCurrency(totalOutflow)}</AppText>
          </View>
        </View>
        <View style={styles.divider} />
        <AppText weight="semibold">Overspending</AppText>
        {overspent.length ? (
          overspent.map((category) => (
            <View key={category.id} style={styles.rowBetween}>
              <View>
                <AppText>{category.name}</AppText>
                <AppText muted>{formatCurrency(category.spent)} spent</AppText>
              </View>
              <AppText weight="semibold" style={styles.negative}>-{formatCurrency(Math.abs(category.remaining))}</AppText>
            </View>
          ))
        ) : (
          <AppText muted>No overspending</AppText>
        )}
      </Card>

      <Card style={styles.cardGap}>
        <AppText weight="semibold" style={styles.cardTitle}>Top Spent Categories</AppText>
        {topSpent.length ? topSpent.map((category) => (
          <View key={category.id} style={styles.rowBetween}>
            <View style={styles.categoryName}>
              <AppText numberOfLines={1}>{category.name}</AppText>
              <AppText muted>{formatCurrency(category.remaining)} left</AppText>
            </View>
            <AppText weight="semibold">{formatCurrency(category.spent)}</AppText>
          </View>
        )) : <AppText muted>No activity this month</AppText>}
      </Card>

      <Card style={styles.cardGap}>
        <AppText weight="semibold" style={styles.cardTitle}>Recent Transactions</AppText>
        {transactions.slice(0, 6).map((txn) => (
          <View key={txn.id} style={styles.rowBetween}>
            <View style={styles.categoryName}>
              <AppText numberOfLines={1}>{txn.payeeName || 'Unknown payee'}</AppText>
              <AppText muted>{formatShortDate(txn.date)} · {txn.accountName}</AppText>
            </View>
            <View style={styles.amountRight}>
              {(txn.inflow ?? 0) > 0 ? <ArrowUpRight size={14} color={colors.success} /> : <ArrowDownLeft size={14} color={colors.muted} />}
              <AppText weight="semibold" style={(txn.inflow ?? 0) > 0 ? styles.positive : undefined}>
                {formatCurrency((txn.inflow ?? 0) || -(txn.outflow ?? 0), { signed: true })}
              </AppText>
            </View>
          </View>
        ))}
      </Card>
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  statGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.md
  },
  statCard: {
    width: '47%',
    gap: spacing.xs
  },
  statValue: {
    fontSize: 20,
    lineHeight: 26
  },
  cardGap: {
    gap: spacing.md
  },
  cardTitle: {
    fontSize: 17
  },
  rowBetween: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  moneyRow: {
    flexDirection: 'row',
    justifyContent: 'space-between'
  },
  divider: {
    height: StyleSheet.hairlineWidth,
    backgroundColor: colors.border
  },
  categoryName: {
    flex: 1
  },
  amountRight: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs
  },
  positive: {
    color: colors.success
  },
  negative: {
    color: colors.danger
  }
});
