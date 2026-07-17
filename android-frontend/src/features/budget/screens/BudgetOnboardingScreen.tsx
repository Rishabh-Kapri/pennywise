import { useState } from 'react';
import { StyleSheet, TextInput, View } from 'react-native';
import { LogOut } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { SectionHeader } from '../../../components/SectionHeader';
import { LoadingState } from '../../../utils/constants';
import { colors, radii, spacing } from '../../../theme';
import { logout } from '../../auth/store/authSlice';
import { budgetTemplates } from '../constants';
import { createBudget, fetchAllBudgets } from '../store/budgetSlice';

export function BudgetOnboardingScreen() {
  const dispatch = useAppDispatch();
  const loading = useAppSelector((state) => state.budgets.loading);
  const error = useAppSelector((state) => state.budgets.error);
  const [name, setName] = useState('Personal Budget');

  const create = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    dispatch(createBudget({ name: trimmed, templateGroups: budgetTemplates }));
  };

  const isPending = loading === LoadingState.PENDING;

  return (
    <Screen style={styles.screen}>
      <SectionHeader title="Create your budget" subtitle="Start with a practical category template — everything is editable later." />

      {error ? (
        <View style={styles.errorCard}>
          <AppText variant="caption" tone="danger">
            {error}
          </AppText>
          <AppText variant="caption" muted>
            If you already have a budget, retry loading instead of creating a new one.
          </AppText>
          <Button variant="secondary" size="sm" disabled={isPending} onPress={() => dispatch(fetchAllBudgets())}>
            {isPending ? 'Retrying...' : 'Retry loading budgets'}
          </Button>
        </View>
      ) : null}

      <View style={styles.field}>
        <AppText variant="label" tone="faint">Budget name</AppText>
        <TextInput
          value={name}
          onChangeText={setName}
          style={styles.input}
          placeholder="Personal Budget"
          placeholderTextColor={colors.faint}
        />
      </View>

      <View style={styles.field}>
        <AppText variant="label" tone="faint">Starter categories</AppText>
        <Card style={styles.templateList}>
          {budgetTemplates.map((group, index) => (
            <View key={group.name} style={[styles.templateRow, index > 0 && styles.rowDivider]}>
              <AppText weight="semibold">{group.name}</AppText>
              <AppText variant="caption" muted>{group.categories.map((category) => category.name).join(', ')}</AppText>
            </View>
          ))}
        </Card>
      </View>

      <Button disabled={isPending} onPress={create}>
        {isPending ? 'Working...' : 'Create budget'}
      </Button>

      <Button variant="ghost" onPress={() => void dispatch(logout())}>
        <View style={styles.logoutContent}>
          <LogOut size={16} color={colors.muted} />
          <AppText weight="semibold" muted>Log out</AppText>
        </View>
      </Button>
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.xl
  },
  errorCard: {
    gap: spacing.md,
    backgroundColor: colors.dangerMuted,
    borderRadius: radii.lg,
    padding: spacing.lg
  },
  field: {
    gap: spacing.sm
  },
  input: {
    minHeight: 50,
    borderRadius: radii.md,
    paddingHorizontal: spacing.lg,
    color: colors.text,
    backgroundColor: colors.surfaceStrong,
    fontSize: 15
  },
  templateList: {
    gap: 0,
    paddingVertical: spacing.xs
  },
  templateRow: {
    gap: spacing.xs,
    paddingVertical: spacing.md
  },
  rowDivider: {
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  logoutContent: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  }
});
