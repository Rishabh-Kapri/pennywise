import { Image, Pressable, StyleSheet, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import { Check, ChevronLeft, LogOut, UserRound } from 'lucide-react-native';
import appConfig from '../../../../app.json';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { IconTile } from '../../../components/IconTile';
import { logout } from '../../auth/store/authSlice';
import { selectAllBudgets, selectSelectedBudget, setSelectedBudget, updateBudgetSelection } from '../../budget/store/budgetSlice';
import { config } from '../../../config/env';
import { colors, spacing } from '../../../theme';

function StatCell({ label, value }: { label: string; value: number | string }) {
  return (
    <View style={styles.statCell}>
      <AppText variant="heading" tabular>{value}</AppText>
      <AppText variant="caption" muted>{label}</AppText>
    </View>
  );
}

export function SettingsScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation();
  const user = useAppSelector((state) => state.auth.user);
  const budgets = useAppSelector(selectAllBudgets);
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const accountCount = useAppSelector((state) => state.accounts.allAccounts.length);
  const payeeCount = useAppSelector((state) => state.payees.allPayees.length);
  const tagCount = useAppSelector((state) => state.tags.tags.length);
  const transactionTotal = useAppSelector((state) => state.transactions.total);

  const chooseBudget = (budgetId?: string) => {
    const budget = budgets.find((item) => item.id === budgetId);
    if (!budget) return;
    dispatch(setSelectedBudget(budget));
    dispatch(updateBudgetSelection({ budget, isSelected: true }));
  };

  return (
    <Screen style={styles.screen}>
      <View style={styles.headerRow}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Go back"
          style={({ pressed }) => [styles.backButton, pressed && styles.pressed]}
          onPress={() => navigation.goBack()}
        >
          <ChevronLeft size={20} color={colors.text} />
        </Pressable>
        <AppText variant="title">Settings</AppText>
      </View>

      <Card style={styles.profileCard}>
        {user?.picture ? (
          <Image source={{ uri: user.picture }} style={styles.profilePhoto} />
        ) : (
          <IconTile size={52}>
            <UserRound color={colors.primary} size={24} />
          </IconTile>
        )}
        <View style={styles.profileMain}>
          <AppText variant="heading">{user?.name ?? 'Pennywise user'}</AppText>
          <AppText variant="caption" muted>{user?.email ?? 'Signed in'}</AppText>
          <AppText variant="caption" tone="faint">Signed in with Google</AppText>
        </View>
      </Card>

      <View style={styles.section}>
        <AppText variant="label" tone="faint" style={styles.sectionLabel}>At a glance</AppText>
        <Card style={styles.statsCard}>
          <StatCell label="Accounts" value={accountCount} />
          <View style={styles.statDivider} />
          <StatCell label="Payees" value={payeeCount} />
          <View style={styles.statDivider} />
          <StatCell label="Tags" value={tagCount} />
          <View style={styles.statDivider} />
          <StatCell label="Transactions" value={transactionTotal} />
        </Card>
      </View>

      <View style={styles.section}>
        <AppText variant="label" tone="faint" style={styles.sectionLabel}>Budgets</AppText>
        <Card style={styles.listCard}>
          {budgets.map((budget, index) => {
            const selected = budget.id === selectedBudget?.id;
            return (
              <Pressable
                key={budget.id ?? budget.name}
                onPress={() => chooseBudget(budget.id)}
                style={({ pressed }) => [styles.budgetRow, index > 0 && styles.rowDivider, pressed && styles.pressed]}
              >
                <View style={styles.budgetMain}>
                  <AppText weight="medium">{budget.name}</AppText>
                  <AppText variant="caption" muted>{selected ? 'Selected budget' : 'Tap to switch'}</AppText>
                </View>
                {selected ? (
                  <View style={styles.selectedBadge}>
                    <Check size={13} color={colors.primary} />
                  </View>
                ) : null}
              </Pressable>
            );
          })}
        </Card>
      </View>

      <View style={styles.section}>
        <AppText variant="label" tone="faint" style={styles.sectionLabel}>App</AppText>
        <Card style={styles.listCard}>
          <View style={styles.infoRow}>
            <AppText variant="caption" muted>Version</AppText>
            <AppText variant="caption" weight="medium" tabular>{appConfig.expo.version}</AppText>
          </View>
          <View style={[styles.infoRow, styles.rowDivider]}>
            <AppText variant="caption" muted>API</AppText>
            <AppText variant="caption" weight="medium" numberOfLines={1} style={styles.infoValue}>
              {config.apiBaseUrl}
            </AppText>
          </View>
          <View style={[styles.infoRow, styles.rowDivider]}>
            <AppText variant="caption" muted>Budget ID</AppText>
            <AppText variant="caption" weight="medium" numberOfLines={1} style={styles.infoValue}>
              {selectedBudget?.id ?? '—'}
            </AppText>
          </View>
        </Card>
      </View>

      <Button variant="danger" onPress={() => void dispatch(logout())}>
        <View style={styles.logoutContent}>
          <LogOut size={17} color={colors.danger} />
          <AppText weight="semibold" tone="danger">Log out</AppText>
        </View>
      </Button>
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.xl
  },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  backButton: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  profileCard: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.lg
  },
  profilePhoto: {
    width: 52,
    height: 52,
    borderRadius: 26
  },
  profileMain: {
    flex: 1,
    gap: 2
  },
  section: {
    gap: spacing.sm
  },
  sectionLabel: {
    marginLeft: spacing.xs
  },
  statsCard: {
    flexDirection: 'row',
    alignItems: 'stretch',
    paddingVertical: spacing.md
  },
  statCell: {
    flex: 1,
    alignItems: 'center',
    gap: 2
  },
  statDivider: {
    width: StyleSheet.hairlineWidth,
    backgroundColor: colors.border
  },
  listCard: {
    paddingVertical: spacing.xs
  },
  budgetRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md,
    paddingVertical: spacing.md
  },
  infoRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.lg,
    paddingVertical: spacing.md
  },
  infoValue: {
    flexShrink: 1,
    textAlign: 'right'
  },
  rowDivider: {
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  budgetMain: {
    flex: 1,
    gap: 1
  },
  selectedBadge: {
    width: 26,
    height: 26,
    borderRadius: 13,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryMuted
  },
  pressed: {
    opacity: 0.7
  },
  logoutContent: {
    flexDirection: 'row',
    gap: spacing.sm,
    alignItems: 'center'
  }
});
