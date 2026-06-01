import { useState } from 'react';
import { StyleSheet, TextInput, View } from 'react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { SectionHeader } from '../../../components/SectionHeader';
import { LoadingState } from '../../../utils/constants';
import { colors, radii, spacing } from '../../../theme';
import { budgetTemplates } from '../constants';
import { createBudget } from '../store/budgetSlice';

export function BudgetOnboardingScreen() {
  const dispatch = useAppDispatch();
  const loading = useAppSelector((state) => state.budgets.loading);
  const [name, setName] = useState('Personal Budget');

  const create = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    dispatch(createBudget({ name: trimmed, templateGroups: budgetTemplates }));
  };

  return (
    <Screen style={styles.screen}>
      <SectionHeader title="Create Budget" subtitle="Start with a practical category template. You can edit everything later." />
      <Card style={styles.card}>
        <AppText weight="semibold">Budget name</AppText>
        <TextInput value={name} onChangeText={setName} style={styles.input} placeholder="Personal Budget" />
        <View style={styles.templateList}>
          {budgetTemplates.map((group) => (
            <View key={group.name}>
              <AppText weight="semibold">{group.name}</AppText>
              <AppText muted>{group.categories.map((category) => category.name).join(', ')}</AppText>
            </View>
          ))}
        </View>
        <Button disabled={loading === LoadingState.PENDING} onPress={create}>
          {loading === LoadingState.PENDING ? 'Creating...' : 'Create budget'}
        </Button>
      </Card>
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  card: {
    gap: spacing.lg
  },
  input: {
    minHeight: 48,
    borderRadius: radii.sm,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.border,
    paddingHorizontal: spacing.md,
    color: colors.text,
    backgroundColor: colors.surface
  },
  templateList: {
    gap: spacing.md
  }
});
