import { useState } from 'react';
import { Pressable, RefreshControl, StyleSheet, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import type { NativeStackNavigationProp } from '@react-navigation/native-stack';
import { CheckCircle2, ChevronDown, ChevronLeft, ChevronRight, ChevronUp, TriangleAlert } from 'lucide-react-native';
import type { RootStackParamList } from '../../../navigation/types';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Card } from '../../../components/Card';
import { CircleButton, LetterTile } from '../../../components/Blocks';
import { Hatch } from '../../../components/Hatch';
import { ProgressBar } from '../../../components/ProgressBar';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { AmountSheet } from '../../../components/AmountKeypad';
import { fetchAllCategoryGroups, fetchInflowAmount, toggleGroupCollapse, updateCategoryBudget } from '../../category/store/categorySlice';
import { selectMonthInHumanFormat, selectSelectedMonth, setSelectedMonth } from '../store/budgetSlice';
import { formatCurrency, shiftMonth } from '../../../utils/date';
import { chartSeries, colors, radii, spacing } from '../../../theme';

type Nav = NativeStackNavigationProp<RootStackParamList>;

/** The assigned amount; tapping opens the calculator sheet. */
function AssignedButton({ value, onPress }: { value: number; onPress: () => void }) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`Assigned ${formatCurrency(value)}. Edit`}
      style={({ pressed }) => [styles.input, pressed && styles.pressed]}
      onPress={onPress}
    >
      <AppText variant="caption" tone="faint">
        Assigned
      </AppText>
      <AppText weight="semibold" tabular style={!value && styles.inputPlaceholder}>
        {value ? formatCurrency(value) : '₹0'}
      </AppText>
    </Pressable>
  );
}

type EditingCategory = { id: string; name: string; budgeted: number; balance: number };

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

function StatTile({ label, value, tone }: { label: string; value: number; tone?: 'danger' | 'success' }) {
  return (
    <View style={styles.statTile}>
      <AppText variant="caption" muted>
        {label}
      </AppText>
      <AppText weight="bold" tabular tone={tone} numberOfLines={1} adjustsFontSizeToFit>
        {formatCurrency(value)}
      </AppText>
    </View>
  );
}

export function BudgetScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation<Nav>();
  const month = useAppSelector(selectSelectedMonth);
  const monthLabel = useAppSelector(selectMonthInHumanFormat);
  const allGroups = useAppSelector((state) => state.categories.allCategoryGroups);
  const inflowAmount = useAppSelector((state) => state.categories.inflowAmount);
  const refreshing = useAppSelector((state) => state.categories.loading === 'pending');
  const [editing, setEditing] = useState<EditingCategory | null>(null);

  // the system (inflow) group is Ready to Assign, shown in the hero instead
  const groups = allGroups.filter((group) => !group.isSystem);
  const assigned = groups.reduce((sum, group) => sum + (group.budgeted?.[month] ?? 0), 0);
  const spent = Math.max(-groups.reduce((sum, group) => sum + (group.activity?.[month] ?? 0), 0), 0);
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

  const over = inflowAmount < 0;
  const done = inflowAmount === 0;

  return (
    <Screen
      style={styles.screen}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={refresh} tintColor={colors.primary} />}
    >
      <View style={styles.headerRow}>
        <AppText variant="title">Budget</AppText>
        <View style={styles.monthStepper}>
          <Pressable accessibilityLabel="Previous month" style={({ pressed }) => [styles.stepButton, pressed && styles.pressed]} onPress={() => moveMonth(-1)}>
            <ChevronLeft size={18} color={colors.text} />
          </Pressable>
          <AppText variant="caption" weight="semibold" style={styles.monthText}>
            {monthLabel}
          </AppText>
          <Pressable accessibilityLabel="Next month" style={({ pressed }) => [styles.stepButton, pressed && styles.pressed]} onPress={() => moveMonth(1)}>
            <ChevronRight size={18} color={colors.text} />
          </Pressable>
        </View>
      </View>

      <View style={styles.hero}>
        <Hatch color={colors.onHero} gap={9} opacity={0.05} />
        <View style={[styles.heroChip, over && styles.heroChipOver]}>
          {over ? <TriangleAlert size={12} color={colors.heroNegative} /> : done ? <CheckCircle2 size={12} color={colors.heroPositive} /> : null}
          <AppText variant="caption" weight="semibold" style={styles.onHero}>
            {over ? 'Assigned more than you have' : done ? 'Every rupee has a job' : 'Ready to assign'}
          </AppText>
        </View>
        <AppText variant="display" tabular style={[styles.onHero, over && { color: colors.heroNegative }]} numberOfLines={1} adjustsFontSizeToFit>
          {formatCurrency(inflowAmount)}
        </AppText>
        <AppText variant="caption" style={styles.onHeroMuted}>
          {monthLabel}
        </AppText>
      </View>

      <View style={styles.statRow}>
        <StatTile label="Assigned" value={assigned} />
        <StatTile label="Spent" value={spent} />
        <StatTile label="Available" value={available} tone={available < 0 ? 'danger' : undefined} />
      </View>

      {groups.map((group, groupIndex) => {
        const color = chartSeries[groupIndex % chartSeries.length];
        const groupBudgeted = group.budgeted?.[month] ?? 0;
        const groupSpent = Math.max(-(group.activity?.[month] ?? 0), 0);
        return (
          <Card key={group.id ?? group.name} style={styles.groupCard}>
            <Pressable style={styles.groupHeader} onPress={() => group.id && dispatch(toggleGroupCollapse(group.id))} accessibilityRole="button">
              <View style={styles.grow}>
                <AppText variant="heading" numberOfLines={1}>
                  {group.name}
                </AppText>
                <AppText variant="caption" muted tabular>
                  {`${formatCurrency(groupSpent)} of ${formatCurrency(groupBudgeted)} · ${formatCurrency(group.balance?.[month] ?? 0)} left`}
                </AppText>
              </View>
              <CircleButton label={group.collapsed ? `Expand ${group.name}` : `Collapse ${group.name}`}>
                {group.collapsed ? <ChevronDown size={16} color={colors.text} /> : <ChevronUp size={16} color={colors.text} />}
              </CircleButton>
            </Pressable>

            {!group.collapsed &&
              group.categories.map((category) => {
                const budgeted = category.budgeted?.[month] ?? 0;
                const categorySpent = Math.max(-(category.activity?.[month] ?? 0), 0);
                const balance = category.balance?.[month] ?? 0;
                const percent = budgeted > 0 ? (categorySpent / budgeted) * 100 : categorySpent > 0 ? 100 : 0;
                return (
                  <View key={category.id ?? category.name} style={styles.categoryRow}>
                    <Pressable
                      accessibilityRole="button"
                      accessibilityLabel={`Open ${category.name}`}
                      style={({ pressed }) => [styles.categoryMain, pressed && styles.pressed]}
                      disabled={!category.id}
                      onPress={() => category.id && navigation.navigate('Category', { id: category.id })}
                    >
                      <LetterTile name={category.name} color={color} size={36} />
                      <View style={styles.categoryText}>
                        <View style={styles.categoryTop}>
                          <AppText weight="semibold" numberOfLines={1} style={styles.grow}>
                            {category.name}
                          </AppText>
                          <AvailablePill amount={balance} />
                        </View>
                        <ProgressBar percent={percent} height={4} color={balance < 0 ? colors.danger : color} />
                        <AppText variant="caption" tone="faint" tabular>
                          {`${formatCurrency(categorySpent)} spent`}
                        </AppText>
                      </View>
                    </Pressable>
                    {category.id ? (
                      <AssignedButton
                        value={budgeted}
                        onPress={() => setEditing({ id: category.id as string, name: category.name, budgeted, balance })}
                      />
                    ) : null}
                  </View>
                );
              })}
          </Card>
        );
      })}

      <AmountSheet
        visible={editing !== null}
        title={editing ? `Assign to ${editing.name}` : ''}
        subtitle={editing ? `${formatCurrency(editing.balance)} available · type a sum like 1200+300` : undefined}
        initialValue={editing?.budgeted ?? 0}
        onClose={() => setEditing(null)}
        onSave={(budgeted) => {
          if (editing && budgeted !== editing.budgeted) {
            dispatch(updateCategoryBudget({ categoryId: editing.id, month, budgeted }));
          }
          setEditing(null);
        }}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.md
  },
  grow: {
    flex: 1,
    minWidth: 0
  },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  monthStepper: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: colors.surface,
    borderRadius: radii.full,
    padding: 3
  },
  stepButton: {
    width: 34,
    height: 34,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong
  },
  monthText: {
    minWidth: 92,
    textAlign: 'center'
  },
  pressed: {
    opacity: 0.6
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
  heroChip: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    borderRadius: radii.full,
    backgroundColor: colors.heroControl,
    paddingHorizontal: spacing.md,
    paddingVertical: 5
  },
  heroChipOver: {
    backgroundColor: 'rgba(255, 138, 138, 0.18)'
  },
  onHero: {
    color: colors.onHero
  },
  onHeroMuted: {
    color: colors.onHeroMuted
  },
  statRow: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  statTile: {
    flex: 1,
    alignItems: 'center',
    gap: 2,
    borderRadius: radii.md,
    backgroundColor: colors.surface,
    paddingVertical: 10,
    paddingHorizontal: spacing.sm
  },
  groupCard: {
    gap: 0,
    paddingVertical: 10
  },
  groupHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingBottom: 6
  },
  categoryRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    paddingVertical: 10,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  categoryMain: {
    flex: 1,
    minWidth: 0,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  categoryText: {
    flex: 1,
    minWidth: 0,
    gap: 5
  },
  categoryTop: {
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
    backgroundColor: colors.surfaceStrong
  },
  pillNeutralText: {
    color: colors.muted
  },
  input: {
    minWidth: 84,
    borderRadius: radii.sm,
    borderWidth: 1,
    borderColor: colors.border,
    alignItems: 'flex-end',
    justifyContent: 'center',
    paddingHorizontal: 10,
    paddingVertical: 5
  },
  inputPlaceholder: {
    color: colors.faint
  }
});
