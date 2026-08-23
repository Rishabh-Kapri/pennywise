import { useEffect, useMemo } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { AlertTriangle, ChevronDown, ChevronRight, Cpu, RefreshCw } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { colors, radii, spacing } from '../../../theme';
import { LoadingState } from '../../../utils/constants';
import { fetchPipelineRunDetail, retryPipelineRun } from '../store/pipelineSlice';
import type { PipelineRun, PipelineRunEvent } from '../types';
import { detailString, formatRelativeTime, formatTokens, groupEventsByEmail, runCounts } from '../utils';
import { EmailCard } from './EmailCard';
import { RunUsage, StatusChip, Stepper } from './RunPieces';

function RunTimeline({ events }: { events: PipelineRunEvent[] }) {
  const runLevelEvents = events.filter((event) => !event.messageId);
  if (runLevelEvents.length === 0) return null;

  return (
    <View style={styles.timeline}>
      {runLevelEvents.map((event) => (
        <View key={event.id} style={styles.timelineRow}>
          <AppText variant="caption" tone="faint" tabular>
            {new Date(event.createdAt).toLocaleTimeString(undefined, {
              hour: '2-digit',
              minute: '2-digit',
              second: '2-digit'
            })}
          </AppText>
          <AppText variant="caption" muted style={styles.timelineStep}>
            {event.step.replace(/_/g, ' ')}
          </AppText>
          <AppText
            variant="caption"
            tone={event.status === 'failed' ? 'danger' : event.status === 'succeeded' ? 'success' : 'muted'}
          >
            {event.status.replace(/_/g, ' ')}
          </AppText>
          {detailString(event, 'error') ? (
            <AppText variant="caption" tone="danger" numberOfLines={2} style={styles.timelineError}>
              {detailString(event, 'error')}
            </AppText>
          ) : null}
        </View>
      ))}
    </View>
  );
}

export function RunCard({
  run,
  isExpanded,
  onToggle,
  onRetryResult
}: {
  run: PipelineRun;
  isExpanded: boolean;
  onToggle: () => void;
  onRetryResult: (message: string) => void;
}) {
  const dispatch = useAppDispatch();
  const events = useAppSelector((state) => state.pipeline.eventsByRunId[run.id]);
  const detailLoading = useAppSelector((state) => state.pipeline.detailLoading);
  const retryingRunId = useAppSelector((state) => state.pipeline.retryingRunId);

  useEffect(() => {
    if (isExpanded) {
      void dispatch(fetchPipelineRunDetail(run.id));
    }
    // refetch the timeline when the run row changes while expanded
  }, [dispatch, isExpanded, run.id, run.updatedAt]);

  const handleRetry = async () => {
    try {
      await dispatch(retryPipelineRun(run.id)).unwrap();
      onRetryResult('Retry signal sent');
    } catch (err: unknown) {
      onRetryResult(err instanceof Error ? err.message : 'Failed to retry run');
    }
  };

  const emailGroups = useMemo(() => groupEventsByEmail(events ?? []), [events]);
  const counts = runCounts(run);
  const isRetrying = retryingRunId === run.id;

  return (
    <Card style={styles.card}>
      <Pressable
        accessibilityRole="button"
        accessibilityState={{ expanded: isExpanded }}
        onPress={onToggle}
        style={({ pressed }) => [styles.header, pressed && styles.pressed]}
      >
        {isExpanded ? (
          <ChevronDown size={15} color={colors.muted} />
        ) : (
          <ChevronRight size={15} color={colors.muted} />
        )}
        <View style={styles.headerMain}>
          <View style={styles.headerTop}>
            <StatusChip status={run.status} />
            <AppText variant="caption" tone="faint" tabular>{formatRelativeTime(run.startedAt)}</AppText>
          </View>
          <AppText weight="medium" numberOfLines={1}>
            {run.emailAccount ?? (run.trigger === 'manual' ? 'manual run' : 'email')}
          </AppText>
          {counts ? <AppText variant="caption" muted>{counts}</AppText> : null}
          <View style={styles.badgeRow}>
            {run.gmailHistoryId !== undefined ? (
              <View style={styles.badge}>
                <AppText variant="caption" tone="faint">history {run.gmailHistoryId}</AppText>
              </View>
            ) : null}
            {run.llmCalls > 0 ? (
              <View style={styles.badge}>
                <Cpu size={11} color={colors.faint} />
                <AppText variant="caption" tone="faint">
                  {formatTokens(run.inputTokens + run.outputTokens)} tokens
                </AppText>
              </View>
            ) : null}
          </View>
        </View>
      </Pressable>

      <Stepper run={run} />

      {run.error && run.status !== 'completed' ? (
        <View style={styles.errorBox}>
          <View style={styles.errorTextRow}>
            <AlertTriangle size={13} color={colors.danger} />
            <AppText variant="caption" tone="danger" style={styles.errorText}>{run.error}</AppText>
          </View>
          {run.status === 'waiting_retry' ? (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="Retry run"
              disabled={isRetrying}
              onPress={() => void handleRetry()}
              style={({ pressed }) => [styles.retryButton, pressed && styles.pressed, isRetrying && styles.disabled]}
            >
              {isRetrying ? (
                <ActivityIndicator size="small" color={colors.primary} />
              ) : (
                <RefreshCw size={13} color={colors.primary} />
              )}
              <AppText variant="caption" weight="semibold" tone="primary">Retry</AppText>
            </Pressable>
          ) : null}
        </View>
      ) : null}

      {isExpanded ? (
        <View style={styles.detail}>
          {events === undefined && detailLoading === LoadingState.PENDING ? (
            <AppText variant="caption" muted>Loading timeline…</AppText>
          ) : (
            <>
              {emailGroups.map((group) => (
                <EmailCard key={group.messageId} group={group} />
              ))}
              <RunUsage run={run} />
              <RunTimeline events={events ?? []} />
              {(events ?? []).length === 0 ? (
                <AppText variant="caption" muted>No timeline recorded for this run.</AppText>
              ) : null}
            </>
          )}
        </View>
      ) : null}
    </Card>
  );
}

const styles = StyleSheet.create({
  card: {
    gap: spacing.md
  },
  header: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: spacing.sm
  },
  headerMain: {
    flex: 1,
    gap: spacing.xs
  },
  headerTop: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.sm
  },
  badgeRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.xs
  },
  badge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong
  },
  errorBox: {
    gap: spacing.sm,
    padding: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.dangerMuted
  },
  errorTextRow: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: spacing.xs
  },
  errorText: {
    flex: 1
  },
  retryButton: {
    alignSelf: 'flex-start',
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: radii.full,
    backgroundColor: colors.primaryMuted
  },
  disabled: {
    opacity: 0.5
  },
  detail: {
    gap: spacing.md,
    paddingTop: spacing.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  timeline: {
    gap: spacing.xs
  },
  timelineRow: {
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: spacing.sm
  },
  timelineStep: {
    textTransform: 'capitalize'
  },
  timelineError: {
    flexBasis: '100%'
  },
  pressed: {
    opacity: 0.7
  }
});
