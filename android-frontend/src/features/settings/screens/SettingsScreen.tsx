import { Pressable, StyleSheet, View } from 'react-native';
import { LogOut, UserRound } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { SectionHeader } from '../../../components/SectionHeader';
import { logout } from '../../auth/store/authSlice';
import { selectAllBudgets, selectSelectedBudget, setSelectedBudget, updateBudgetSelection } from '../../budget/store/budgetSlice';
import { colors, spacing } from '../../../theme';

export function SettingsScreen() {
  const dispatch = useAppDispatch();
  const user = useAppSelector((state) => state.auth.user);
  const budgets = useAppSelector(selectAllBudgets);
  const selectedBudget = useAppSelector(selectSelectedBudget);

  const chooseBudget = (budgetId?: string) => {
    const budget = budgets.find((item) => item.id === budgetId);
    if (!budget) return;
    dispatch(setSelectedBudget(budget));
    dispatch(updateBudgetSelection({ budget, isSelected: true }));
  };

  return (
    <Screen style={styles.screen}>
      <SectionHeader title="Settings" subtitle="Account, budget switching, and local session controls." />

      <Card style={styles.profileCard}>
        <View style={styles.avatar}>
          <UserRound color={colors.primary} size={24} />
        </View>
        <View style={styles.profileMain}>
          <AppText weight="semibold">{user?.name ?? 'Pennywise user'}</AppText>
          <AppText muted>{user?.email ?? 'Signed in'}</AppText>
        </View>
      </Card>

      <Card style={styles.cardGap}>
        <AppText weight="semibold" style={styles.cardTitle}>Budgets</AppText>
        {budgets.map((budget) => {
          const selected = budget.id === selectedBudget?.id;
          return (
            <Pressable key={budget.id ?? budget.name} onPress={() => chooseBudget(budget.id)} style={styles.budgetRow}>
              <View>
                <AppText weight="semibold">{budget.name}</AppText>
                <AppText muted>{selected ? 'Selected budget' : 'Tap to switch'}</AppText>
              </View>
              {selected ? <View style={styles.selectedDot} /> : null}
            </Pressable>
          );
        })}
      </Card>

      <Button variant="danger" onPress={() => void dispatch(logout())}>
        <View style={styles.logoutContent}>
          <LogOut size={18} color="#fff" />
          <AppText weight="semibold" style={styles.logoutLabel}>Log out</AppText>
        </View>
      </Button>
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  profileCard: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  avatar: {
    width: 52,
    height: 52,
    borderRadius: 26,
    backgroundColor: colors.primaryLight,
    alignItems: 'center',
    justifyContent: 'center'
  },
  profileMain: {
    flex: 1
  },
  cardGap: {
    gap: spacing.md
  },
  cardTitle: {
    fontSize: 18
  },
  budgetRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: spacing.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  selectedDot: {
    width: 10,
    height: 10,
    borderRadius: 5,
    backgroundColor: colors.primary
  },
  logoutContent: {
    flexDirection: 'row',
    gap: spacing.sm,
    alignItems: 'center'
  },
  logoutLabel: {
    color: '#fff'
  }
});
