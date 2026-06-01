import { useState } from 'react';
import { Pressable, StyleSheet, TextInput, View } from 'react-native';
import { ChevronDown, ChevronLeft, ChevronRight, ChevronUp, Check } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { SectionHeader } from '../../../components/SectionHeader';
import { fetchAllCategoryGroups, toggleGroupCollapse, updateCategoryBudget } from '../../category/store/categorySlice';
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
      style={styles.input}
    />
  );
}

export function BudgetScreen() {
  const dispatch = useAppDispatch();
  const month = useAppSelector(selectSelectedMonth);
  const monthLabel = useAppSelector(selectMonthInHumanFormat);
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  const inflowAmount = useAppSelector((state) => state.categories.inflowAmount);

  const assigned = groups.reduce((sum, group) => sum + (group.budgeted?.[month] ?? 0), 0);
  const activity = groups.reduce((sum, group) => sum + (group.activity?.[month] ?? 0), 0);
  const available = groups.reduce((sum, group) => sum + (group.balance?.[month] ?? 0), 0);

  const moveMonth = (delta: number) => {
    const next = shiftMonth(month, delta);
    dispatch(setSelectedMonth(next));
    dispatch(fetchAllCategoryGroups(next));
  };

  return (
    <Screen style={styles.screen}>
      <SectionHeader title="Budget" subtitle="Assign inflow, track activity, and keep available money visible." />

      <View style={styles.monthRow}>
        <Button variant="secondary" onPress={() => moveMonth(-1)}>
          <ChevronLeft size={16} color={colors.text} />
        </Button>
        <AppText weight="bold" style={styles.monthText}>{monthLabel}</AppText>
        <Button variant="secondary" onPress={() => moveMonth(1)}>
          <ChevronRight size={16} color={colors.text} />
        </Button>
      </View>

      <Card style={[styles.assignCard, inflowAmount === 0 ? styles.assignedCard : undefined]}>
        <View>
          <AppText weight="bold" style={styles.readyAmount}>{formatCurrency(inflowAmount)}</AppText>
          <AppText muted>{inflowAmount === 0 ? 'All assigned' : 'Ready to assign'}</AppText>
        </View>
        {inflowAmount === 0 ? <Check color={colors.success} size={24} /> : null}
      </Card>

      <View style={styles.summaryRow}>
        <Card style={styles.summaryCard}>
          <AppText muted>Assigned</AppText>
          <AppText weight="bold">{formatCurrency(assigned)}</AppText>
        </Card>
        <Card style={styles.summaryCard}>
          <AppText muted>Activity</AppText>
          <AppText weight="bold">{formatCurrency(activity)}</AppText>
        </Card>
        <Card style={styles.summaryCard}>
          <AppText muted>Available</AppText>
          <AppText weight="bold">{formatCurrency(available)}</AppText>
        </Card>
      </View>

      {groups.map((group) => (
        <Card key={group.id ?? group.name} style={styles.groupCard}>
          <Pressable style={styles.groupHeader} onPress={() => group.id && dispatch(toggleGroupCollapse(group.id))}>
            <View style={styles.groupTitle}>
              {group.collapsed ? <ChevronDown size={18} color={colors.muted} /> : <ChevronUp size={18} color={colors.muted} />}
              <AppText weight="semibold">{group.name}</AppText>
            </View>
            <AppText weight="semibold">{formatCurrency(group.balance?.[month] ?? 0)}</AppText>
          </Pressable>

          {!group.collapsed && group.categories.map((category) => {
            const budgeted = category.budgeted?.[month] ?? 0;
            const spent = category.activity?.[month] ?? 0;
            const balance = category.balance?.[month] ?? 0;
            return (
              <View key={category.id ?? category.name} style={styles.categoryRow}>
                <View style={styles.categoryName}>
                  <AppText numberOfLines={1}>{category.name}</AppText>
                  <AppText muted>{formatCurrency(spent)} activity · {formatCurrency(balance)} left</AppText>
                </View>
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
  monthRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  monthText: {
    flex: 1,
    textAlign: 'center',
    fontSize: 18
  },
  assignCard: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between'
  },
  assignedCard: {
    borderColor: colors.success,
    backgroundColor: colors.surfaceStrong
  },
  readyAmount: {
    fontSize: 26,
    lineHeight: 32
  },
  summaryRow: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  summaryCard: {
    flex: 1,
    padding: spacing.md
  },
  groupCard: {
    gap: spacing.sm
  },
  groupHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center'
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
    paddingVertical: spacing.sm,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  categoryName: {
    flex: 1
  },
  input: {
    width: 86,
    minHeight: 38,
    borderRadius: radii.sm,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.border,
    backgroundColor: colors.surface,
    textAlign: 'right',
    paddingHorizontal: spacing.sm,
    color: colors.text
  }
});
