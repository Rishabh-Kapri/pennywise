import { RefreshControl, StyleSheet, View } from 'react-native';
import { ArrowDownLeft, ArrowUpRight, Landmark, WalletCards } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { IconTile } from '../../../components/IconTile';
import { InitialAvatar } from '../../../components/InitialAvatar';
import { ProgressBar } from '../../../components/ProgressBar';
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
      <View style={styles.header}>
        <AppText variant="caption" muted>
          {new Intl.DateTimeFormat('en-IN', { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date())}
        </AppText>
        <AppText variant="title">{`${greeting()}${user?.name ? `, ${user.name.split(' ')[0]}` : ''}`}</AppText>
      </View>

      <View style={styles.hero}>
        <AppText variant="label" tone="faint">Total balance</AppText>
        <AppText variant="display" tabular>{formatCurrency(totalBalance)}</AppText>
        <View style={styles.heroChips}>
          <View style={styles.heroChip}>
            <IconTile tone="success" size={32}>
              <WalletCards size={16} color={colors.success} />
            </IconTile>
            <View>
              <AppText variant="caption" muted>Cash</AppText>
              <AppText variant="caption" weight="semibold" tabular>{formatCurrency(availableCash)}</AppText>
            </View>
          </View>
          <View style={styles.heroChip}>
            <IconTile tone="danger" size={32}>
              <Landmark size={16} color={colors.danger} />
            </IconTile>
            <View>
              <AppText variant="caption" muted>Debt</AppText>
              <AppText variant="caption" weight="semibold" tabular>{formatCurrency(debt)}</AppText>
            </View>
          </View>
          <View style={styles.heroChip}>
            <IconTile tone="neutral" size={32}>
              <AppText variant="caption" weight="semibold" muted>{accounts.length}</AppText>
            </IconTile>
            <View>
              <AppText variant="caption" muted>Accounts</AppText>
              <AppText variant="caption" weight="semibold">Linked</AppText>
            </View>
          </View>
        </View>
      </View>

      <Card style={styles.cardGap}>
        <View style={styles.rowBetween}>
          <AppText variant="heading">This month</AppText>
          <AppText variant="caption" muted>{monthLabel || 'Current'}</AppText>
        </View>
        <View style={styles.flowRow}>
          <View style={styles.flowCell}>
            <View style={styles.flowLabel}>
              <ArrowUpRight size={14} color={colors.success} />
              <AppText variant="caption" muted>Incoming</AppText>
            </View>
            <AppText variant="heading" tone="success" tabular>+{formatCurrency(totalInflow)}</AppText>
          </View>
          <View style={styles.flowDivider} />
          <View style={styles.flowCell}>
            <View style={styles.flowLabel}>
              <ArrowDownLeft size={14} color={colors.danger} />
              <AppText variant="caption" muted>Outgoing</AppText>
            </View>
            <AppText variant="heading" tone="danger" tabular>-{formatCurrency(totalOutflow)}</AppText>
          </View>
        </View>
        {overspent.length ? (
          <View style={styles.overspendBlock}>
            <AppText variant="label" tone="faint">Overspending</AppText>
            {overspent.map((category) => (
              <View key={category.id} style={styles.rowBetween}>
                <View style={styles.rowMain}>
                  <AppText numberOfLines={1}>{category.name}</AppText>
                  <AppText variant="caption" muted>{formatCurrency(category.spent)} spent</AppText>
                </View>
                <AppText weight="semibold" tone="danger" tabular>-{formatCurrency(Math.abs(category.remaining))}</AppText>
              </View>
            ))}
          </View>
        ) : null}
      </Card>

      <Card style={styles.cardGap}>
        <AppText variant="heading">Top categories</AppText>
        {topSpent.length ? (
          topSpent.map((category) => {
            const total = category.spent + Math.max(category.remaining, 0);
            const percent = total > 0 ? (category.spent / total) * 100 : 100;
            return (
              <View key={category.id} style={styles.categoryBlock}>
                <View style={styles.rowBetween}>
                  <AppText numberOfLines={1} style={styles.rowMain}>{category.name}</AppText>
                  <AppText variant="caption" weight="semibold" tabular>{formatCurrency(category.spent)}</AppText>
                </View>
                <ProgressBar percent={percent} color={category.remaining < 0 ? colors.danger : colors.primary} height={5} />
                <AppText variant="caption" tone="faint" tabular>{formatCurrency(category.remaining)} left</AppText>
              </View>
            );
          })
        ) : (
          <AppText variant="caption" muted>No activity this month</AppText>
        )}
      </Card>

      <Card style={styles.cardGap}>
        <AppText variant="heading">Recent transactions</AppText>
        {transactions.slice(0, 6).map((txn) => {
          const amount = (txn.inflow ?? 0) || -(txn.outflow ?? 0);
          return (
            <View key={txn.id} style={styles.txnRow}>
              <InitialAvatar name={txn.payeeName || '?'} size={38} />
              <View style={styles.rowMain}>
                <AppText weight="medium" numberOfLines={1}>{txn.payeeName || 'Unknown payee'}</AppText>
                <AppText variant="caption" muted numberOfLines={1}>{formatShortDate(txn.date)} · {txn.accountName}</AppText>
              </View>
              <AppText weight="semibold" tone={amount > 0 ? 'success' : 'default'} tabular>
                {formatCurrency(amount, { signed: true })}
              </AppText>
            </View>
          );
        })}
      </Card>
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.xl
  },
  header: {
    gap: spacing.xs
  },
  hero: {
    gap: spacing.sm
  },
  heroChips: {
    flexDirection: 'row',
    gap: spacing.sm,
    marginTop: spacing.sm
  },
  heroChip: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    backgroundColor: colors.surface,
    borderRadius: 16,
    padding: spacing.sm + 2
  },
  cardGap: {
    gap: spacing.lg
  },
  rowBetween: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  rowMain: {
    flex: 1
  },
  flowRow: {
    flexDirection: 'row',
    alignItems: 'center'
  },
  flowCell: {
    flex: 1,
    gap: spacing.xs
  },
  flowLabel: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs
  },
  flowDivider: {
    width: StyleSheet.hairlineWidth,
    alignSelf: 'stretch',
    backgroundColor: colors.border,
    marginHorizontal: spacing.lg
  },
  overspendBlock: {
    gap: spacing.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border,
    paddingTop: spacing.lg
  },
  categoryBlock: {
    gap: spacing.sm
  },
  txnRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  }
});
