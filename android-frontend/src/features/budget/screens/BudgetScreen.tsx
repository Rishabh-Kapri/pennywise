import { useState } from 'react';
import { Pressable, RefreshControl, StyleSheet, TextInput, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import { ChevronDown, ChevronLeft, ChevronRight, ChevronUp, Check } from 'lucide-react-native';
import type { AppTabParamList } from '../../../navigation/types';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { fetchAllCategoryGroups, fetchInflowAmount, toggleGroupCollapse, updateCategoryBudget } from '../../category/store/categorySlice';
import { selectMonthInHumanFormat, selectSelectedMonth, setSelectedMonth } from '../store/budgetSlice';
import { formatCurrency, shiftMonth } from '../../../utils/date';
import { colors, radii, spacing } from '../../../theme';

function BudgetInput({ categoryId, value, month }: { categoryId: string; value: number; month: string }) {
  const dispatch = useAppDispatch();
  const [text, setText] = useState(value ? String(value) : '');

  const commit = () => {
    const next = Number(text.replace(/,/g, ''));
    if (!Number.isFinite(next) || next === value) return;
    dispatch(updateCategoryBudget({ categoryId, month, budgeted: next }));
  };

  return (
    <TextInput
      value={text}
      onChangeText={setText}
      onBlur={commit}
      onSubmitEditing={commit}
      keyboardType="numeric"
      placeholder="0"
      placeholderTextColor={colors.faint}
      style={styles.input}
    />
  );
}

function AvailablePill({ amount }: { amount: number }) {
  const tone = amount < 0 ? styles.pillNegative : amount > 0 ? styles.pillPositive : styles.pillNeutral;
  const toneText = amount < 0 ? styles.pillNegativeText : amount > 0 ? styles.pillPositiveText : styles.pillNeutralText;

  return (
    <View style={[styles.pill, tone]}>
      <AppText variant="caption" weight="semibold" tabular style={toneText}>
        {formatCurrency(amount)}
      </AppText>
    </View>
  );
}

export function BudgetScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation<BottomTabNavigationProp<AppTabParamList>>();
  const month = useAppSelector(selectSelectedMonth);
  const monthLabel = useAppSelector(selectMonthInHumanFormat);
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  const inflowAmount = useAppSelector((state) => state.categories.inflowAmount);
  const refreshing = useAppSelector((state) => state.categories.loading === 'pending');

  const assigned = groups.reduce((sum, group) => sum + (group.budgeted?.[month] ?? 0), 0);
  const activity = groups.reduce((sum, group) => sum + (group.activity?.[month] ?? 0), 0);
  const available = groups.reduce((sum, group) => sum + (group.balance?.[month] ?? 0), 0);

  const moveMonth = (delta: number) => {
    const next = shiftMonth(month, delta);
    dispatch(setSelectedMonth(next));
    dispatch(fetchAllCategoryGroups(next));
  };

  // Assigned/available are derived from the category groups, and "ready to
  // assign" comes from the inflow amount, so both have to be refetched together
  // or the header totals disagree with the rows under them.
  const refresh = () => {
    if (month) dispatch(fetchAllCategoryGroups(month));
    dispatch(fetchInflowAmount());
  };

  return (
    <Screen
      style={styles.screen}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={refresh} tintColor={colors.primary} />}
    >
      <View style={styles.headerRow}>
        <AppText variant="title">Budget</AppText>
        <View style={styles.monthStepper}>
          <Pressable style={({ pressed }) => [styles.stepButton, pressed && styles.pressed]} onPress={() => moveMonth(-1)}>
            <ChevronLeft size={18} color={colors.muted} />
          </Pressable>
          <AppText variant="caption" weight="semibold" style={styles.monthText}>{monthLabel}</AppText>
          <Pressable style={({ pressed }) => [styles.stepButton, pressed && styles.pressed]} onPress={() => moveMonth(1)}>
            <ChevronRight size={18} color={colors.muted} />
          </Pressable>
        </View>
      </View>

      <View style={[styles.assignCard, inflowAmount === 0 && styles.assignCardDone]}>
        <View style={styles.assignMain}>
          <AppText variant="label" tone={inflowAmount === 0 ? 'success' : 'primary'}>
            {inflowAmount === 0 ? 'All assigned' : 'Ready to assign'}
          </AppText>
          <AppText variant="display" tabular style={styles.assignAmount}>{formatCurrency(inflowAmount)}</AppText>
        </View>
        {inflowAmount === 0 ? <Check color={colors.success} size={28} /> : null}
      </View>

      <View style={styles.summaryRow}>
        <View style={styles.summaryCell}>
          <AppText variant="caption" muted>Assigned</AppText>
          <AppText weight="semibold" tabular>{formatCurrency(assigned)}</AppText>
        </View>
        <View style={styles.summaryDivider} />
        <View style={styles.summaryCell}>
          <AppText variant="caption" muted>Activity</AppText>
          <AppText weight="semibold" tabular>{formatCurrency(activity)}</AppText>
        </View>
        <View style={styles.summaryDivider} />
        <View style={styles.summaryCell}>
          <AppText variant="caption" muted>Available</AppText>
          <AppText weight="semibold" tabular>{formatCurrency(available)}</AppText>
        </View>
      </View>

      {groups.map((group) => (
        <Card key={group.id ?? group.name} style={styles.groupCard}>
          <Pressable style={styles.groupHeader} onPress={() => group.id && dispatch(toggleGroupCollapse(group.id))}>
            <View style={styles.groupTitle}>
              <AppText variant="heading">{group.name}</AppText>
              {group.collapsed ? <ChevronDown size={16} color={colors.faint} /> : <ChevronUp size={16} color={colors.faint} />}
            </View>
            <AppText variant="caption" weight="semibold" muted tabular>{formatCurrency(group.balance?.[month] ?? 0)}</AppText>
          </Pressable>

          {!group.collapsed && group.categories.map((category) => {
            const budgeted = category.budgeted?.[month] ?? 0;
            const spent = category.activity?.[month] ?? 0;
            const balance = category.balance?.[month] ?? 0;
            return (
              <View key={category.id ?? category.name} style={styles.categoryRow}>
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel={`View ${category.name} transactions`}
                  style={({ pressed }) => [styles.categoryMain, pressed && styles.pressed]}
                  disabled={!category.id}
                  onPress={() =>
                    category.id &&
                    navigation.navigate('Transactions', {
                      categoryId: category.id,
                      categoryName: category.name,
                      month
                    })
                  }
                >
                  <AppText numberOfLines={1}>{category.name}</AppText>
                  <View style={styles.categoryMeta}>
                    <AppText variant="caption" tone="faint" tabular>{formatCurrency(spent)} spent</AppText>
                    <AvailablePill amount={balance} />
                  </View>
                </Pressable>
                {category.id ? <BudgetInput categoryId={category.id} month={month} value={budgeted} /> : null}
              </View>
            );
          })}
        </Card>
      ))}
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md,
    marginBottom: spacing.xs
  },
  monthStepper: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: colors.surface,
    borderRadius: radii.full,
    paddingHorizontal: spacing.xs
  },
  stepButton: {
    width: 36,
    height: 36,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: radii.full
  },
  monthText: {
    minWidth: 92,
    textAlign: 'center'
  },
  pressed: {
    opacity: 0.6
  },
  assignCard: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md,
    backgroundColor: colors.primaryMuted,
    borderRadius: radii.lg,
    padding: spacing.xl
  },
  assignCardDone: {
    backgroundColor: colors.successMuted
  },
  assignMain: {
    gap: spacing.xs
  },
  assignAmount: {
    fontSize: 32,
    lineHeight: 38
  },
  summaryRow: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: colors.surface,
    borderRadius: radii.lg,
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.lg
  },
  summaryCell: {
    flex: 1,
    gap: 2
  },
  summaryDivider: {
    width: StyleSheet.hairlineWidth,
    alignSelf: 'stretch',
    backgroundColor: colors.border,
    marginHorizontal: spacing.md
  },
  groupCard: {
    gap: spacing.sm
  },
  groupHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: spacing.md,
    minHeight: 32
  },
  groupTitle: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    flex: 1
  },
  categoryRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingVertical: spacing.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  categoryMain: {
    flex: 1,
    gap: spacing.xs,
    paddingVertical: spacing.xs
  },
  categoryMeta: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  pill: {
    borderRadius: radii.full,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2
  },
  pillPositive: {
    backgroundColor: colors.successMuted
  },
  pillPositiveText: {
    color: colors.success
  },
  pillNegative: {
    backgroundColor: colors.dangerMuted
  },
  pillNegativeText: {
    color: colors.danger
  },
  pillNeutral: {
    backgroundColor: colors.surfaceTertiary
  },
  pillNeutralText: {
    color: colors.muted
  },
  input: {
    width: 88,
    minHeight: 40,
    borderRadius: radii.sm,
    backgroundColor: colors.surfaceStrong,
    textAlign: 'right',
    paddingHorizontal: spacing.md,
    color: colors.text,
    fontVariant: ['tabular-nums']
  }
});
