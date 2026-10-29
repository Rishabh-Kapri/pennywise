import { useEffect, useState } from 'react';
import { Pressable, RefreshControl, ScrollView, StyleSheet, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import type { NativeStackNavigationProp } from '@react-navigation/native-stack';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Screen } from '../../../components/Screen';
import type { RootStackParamList } from '../../../navigation/types';
import { colors, radii, spacing } from '../../../theme';
import { LoadingState } from '../../../utils/constants';
import { getSelectedMonthInHumanFormat } from '../../../utils/date';
import { IncomeReport, InvestingReport, NetWorthReport, OverviewReport, SpendingReport } from '../components/ReportViews';
import {
  fetchCashFlowReport,
  fetchNetWorthReport,
  invalidateReports,
  loadGroupTreatments,
  setRangePreset,
  type RangePreset
} from '../store/reportSlice';
import type { ReportTab } from '../types';

const TABS: { key: ReportTab; label: string }[] = [
  { key: 'overview', label: 'Overview' },
  { key: 'spending', label: 'Spending' },
  { key: 'income', label: 'Income' },
  { key: 'investing', label: 'Investing' },
  { key: 'networth', label: 'Net worth' }
];

const PRESETS: { key: RangePreset; label: string }[] = [
  { key: '3m', label: '3M' },
  { key: '6m', label: '6M' },
  { key: '12m', label: '12M' },
  { key: 'ytd', label: 'YTD' },
  { key: 'all', label: 'All' }
];

export function ReportsScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const [tab, setTab] = useState<ReportTab>('overview');
  const { range, preset, cashFlowLoading, netWorthLoading, error } = useAppSelector((state) => state.reports);
  const budgetId = useAppSelector((state) => state.budgets.selectedBudget?.id);

  useEffect(() => {
    dispatch(loadGroupTreatments());
  }, [dispatch]);

  // A range change (or a budget switch) resets the reports to IDLE, so only
  // what the visible tab needs is fetched; the four cash-flow tabs share one.
  useEffect(() => {
    if (!budgetId) return;
    if (tab === 'networth') {
      if (netWorthLoading === LoadingState.IDLE) dispatch(fetchNetWorthReport(range));
    } else if (cashFlowLoading === LoadingState.IDLE) {
      dispatch(fetchCashFlowReport(range));
    }
  }, [dispatch, tab, range, cashFlowLoading, netWorthLoading, budgetId]);

  const refreshing = (tab === 'networth' ? netWorthLoading : cashFlowLoading) === LoadingState.PENDING;

  return (
    <Screen
      style={styles.screen}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={() => dispatch(invalidateReports())} tintColor={colors.primary} />
      }
    >
      <View>
        <AppText variant="title">Reports</AppText>
        <AppText variant="caption" muted>
          {getSelectedMonthInHumanFormat(range.startMonth)} – {getSelectedMonthInHumanFormat(range.endMonth)}
        </AppText>
      </View>

      <View style={styles.presets}>
        {PRESETS.map((option) => {
          const active = option.key === preset;
          return (
            <Pressable
              key={option.key}
              style={[styles.preset, active && styles.presetActive]}
              onPress={() => dispatch(setRangePreset(option.key))}
            >
              <AppText variant="caption" weight="semibold" style={active ? styles.activeText : styles.inactiveText}>
                {option.label}
              </AppText>
            </Pressable>
          );
        })}
      </View>

      <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.tabs}>
        {TABS.map((option) => {
          const active = option.key === tab;
          return (
            <Pressable key={option.key} style={[styles.tab, active && styles.tabActive]} onPress={() => setTab(option.key)}>
              <AppText weight="semibold" style={active ? styles.activeText : styles.inactiveText}>
                {option.label}
              </AppText>
            </Pressable>
          );
        })}
      </ScrollView>

      {error ? (
        <AppText variant="caption" tone="danger">
          {error}
        </AppText>
      ) : null}

      {tab === 'overview' ? <OverviewReport /> : null}
      {tab === 'spending' ? <SpendingReport /> : null}
      {tab === 'income' ? <IncomeReport /> : null}
      {tab === 'investing' ? <InvestingReport onOpenLoans={() => navigation.navigate('Loans')} /> : null}
      {tab === 'networth' ? <NetWorthReport /> : null}
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  presets: {
    flexDirection: 'row',
    backgroundColor: colors.surface,
    borderRadius: radii.full,
    padding: 3
  },
  preset: {
    flex: 1,
    minHeight: 32,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: radii.full
  },
  presetActive: {
    backgroundColor: colors.primaryMuted
  },
  tabs: {
    gap: spacing.lg
  },
  tab: {
    paddingVertical: spacing.sm,
    borderBottomWidth: 2,
    borderBottomColor: 'transparent'
  },
  tabActive: {
    borderBottomColor: colors.primary
  },
  activeText: {
    color: colors.primary
  },
  inactiveText: {
    color: colors.muted
  }
});
