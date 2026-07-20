import { Pressable, StyleSheet, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import { Check, ChevronLeft, LogOut, UserRound } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { IconTile } from '../../../components/IconTile';
import { logout } from '../../auth/store/authSlice';
import { selectAllBudgets, selectSelectedBudget, setSelectedBudget, updateBudgetSelection } from '../../budget/store/budgetSlice';
import { colors, spacing } from '../../../theme';

export function SettingsScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation();
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
        <IconTile size={52}>
          <UserRound color={colors.primary} size={24} />
        </IconTile>
        <View style={styles.profileMain}>
          <AppText variant="heading">{user?.name ?? 'Pennywise user'}</AppText>
          <AppText variant="caption" muted>{user?.email ?? 'Signed in'}</AppText>
        </View>
      </Card>

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
