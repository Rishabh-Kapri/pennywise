import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, RefreshControl, StyleSheet, ToastAndroid, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import { Activity as ActivityIcon, ChevronLeft } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { EmptyState } from '../../../components/EmptyState';
import { Screen } from '../../../components/Screen';
import { colors, radii, spacing } from '../../../theme';
import { LoadingState } from '../../../utils/constants';
import { RunCard } from '../components/RunCard';
import {
  fetchPipelineRuns,
  selectActivePipelineRunCount,
  selectPipelineError,
  selectPipelineRuns,
  selectPipelineRunsLoading
} from '../store/pipelineSlice';

export function ActivityScreen() {
  const dispatch = useAppDispatch();
  const navigation = useNavigation();
  const runs = useAppSelector(selectPipelineRuns);
  const runsLoading = useAppSelector(selectPipelineRunsLoading);
  const error = useAppSelector(selectPipelineError);
  const activeCount = useAppSelector(selectActivePipelineRunCount);
  const [expandedRunId, setExpandedRunId] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  useEffect(() => {
    void dispatch(fetchPipelineRuns());
  }, [dispatch]);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await dispatch(fetchPipelineRuns());
    setRefreshing(false);
  }, [dispatch]);

  const showToast = useCallback((message: string) => {
    ToastAndroid.show(message, ToastAndroid.SHORT);
  }, []);

  return (
    <Screen
      style={styles.screen}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={() => void onRefresh()} tintColor={colors.primary} />
      }
    >
      <View style={styles.headerRow}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Go back"
          style={({ pressed }) => [styles.backButton, pressed && styles.pressed]}
          onPress={() => navigation.goBack()}
        >
          <ChevronLeft size={20} color={colors.text} />
        </Pressable>
        <View style={styles.headerMain}>
          <AppText variant="title">Activity</AppText>
          <AppText variant="caption" muted>
            Transaction emails moving through fetch, extraction, prediction and creation
          </AppText>
        </View>
        {activeCount > 0 ? (
          <View style={styles.activeBadge}>
            <ActivityIndicator size="small" color={colors.primary} />
            <AppText variant="caption" tone="primary" weight="semibold">{activeCount}</AppText>
          </View>
        ) : null}
      </View>

      {runsLoading === LoadingState.PENDING && runs.length === 0 ? (
        <Card>
          <AppText variant="caption" muted>Loading runs…</AppText>
        </Card>
      ) : null}

      {runsLoading === LoadingState.ERROR ? (
        <Card style={styles.errorCard}>
          <AppText variant="caption" tone="danger">{error}</AppText>
        </Card>
      ) : null}

      {runsLoading === LoadingState.SUCCESS && runs.length === 0 ? (
        <EmptyState
          icon={<ActivityIcon color={colors.primary} size={24} />}
          title="No pipeline runs yet"
          body="When a transaction email arrives, its progress will show up here."
        />
      ) : null}

      <View style={styles.runList}>
        {runs.map((run) => (
          <RunCard
            key={run.id}
            run={run}
            isExpanded={expandedRunId === run.id}
            onToggle={() => setExpandedRunId((prev) => (prev === run.id ? null : run.id))}
            onRetryResult={showToast}
          />
        ))}
      </View>
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
    gap: spacing.md
  },
  headerMain: {
    flex: 1,
    gap: 2
  },
  backButton: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  activeBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.xs,
    borderRadius: radii.full,
    backgroundColor: colors.primaryMuted
  },
  errorCard: {
    backgroundColor: colors.dangerMuted
  },
  runList: {
    gap: spacing.md
  },
  pressed: {
    opacity: 0.7
  }
});
