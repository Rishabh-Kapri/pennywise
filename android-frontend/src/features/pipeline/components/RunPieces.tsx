import { ActivityIndicator, StyleSheet, View } from 'react-native';
import { AlertTriangle, CheckCircle2, Cpu, XCircle } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { colors, radii, spacing } from '../../../theme';
import type { LLMCall, PipelineRun } from '../types';
import { STATUS_LABELS, STEPPER, callSummary, formatDuration, formatTokens, stepState } from '../utils';

export function StatusChip({ status }: { status: PipelineRun['status'] }) {
  const tone =
    status === 'completed'
      ? { bg: colors.successMuted, fg: colors.success }
      : status === 'failed'
        ? { bg: colors.dangerMuted, fg: colors.danger }
        : status === 'waiting_retry'
          ? { bg: colors.warningMuted, fg: colors.warning }
          : { bg: colors.primaryMuted, fg: colors.primary };

  return (
    <View style={[styles.chip, { backgroundColor: tone.bg }]}>
      {status === 'running' ? <ActivityIndicator size="small" color={tone.fg} /> : null}
      {status === 'waiting_retry' ? <AlertTriangle size={13} color={tone.fg} /> : null}
      {status === 'completed' ? <CheckCircle2 size={13} color={tone.fg} /> : null}
      {status === 'failed' ? <XCircle size={13} color={tone.fg} /> : null}
      <AppText variant="caption" weight="semibold" style={{ color: tone.fg }}>
        {STATUS_LABELS[status]}
      </AppText>
    </View>
  );
}

const STEP_COLORS: Record<string, string> = {
  done: colors.success,
  active: colors.primary,
  waiting: colors.warning,
  error: colors.danger,
  pending: colors.surfaceTertiary
};

export function Stepper({ run }: { run: PipelineRun }) {
  return (
    <View style={styles.stepper}>
      {STEPPER.map((step, index) => {
        const state = stepState(run, step.key);
        const color = STEP_COLORS[state];
        return (
          <View key={step.key} style={styles.stepItem}>
            {index > 0 ? <View style={[styles.connector, { backgroundColor: color }]} /> : null}
            <View style={[styles.dot, { backgroundColor: color }]} />
            <AppText variant="caption" tone={state === 'pending' ? 'faint' : 'muted'}>
              {step.label}
            </AppText>
          </View>
        );
      })}
    </View>
  );
}

/** Model/token line under a step, with failed fallback attempts called out. */
export function StepUsage({ label, calls }: { label: string; calls: LLMCall[] }) {
  if (calls.length === 0) return null;
  const failed = calls.filter((call) => call.failed);

  return (
    <View style={styles.usageRow}>
      <Cpu size={12} color={colors.faint} />
      <AppText variant="caption" tone="faint">{label}</AppText>
      <AppText variant="caption" muted style={styles.usageSummary} numberOfLines={2}>
        {callSummary(calls)}
      </AppText>
      {failed.length > 0 ? (
        <AppText variant="caption" tone="danger">{failed.length} failed over</AppText>
      ) : null}
    </View>
  );
}

/** Per-model breakdown for the whole run. */
export function RunUsage({ run }: { run: PipelineRun }) {
  const models = Object.entries(run.llmUsage ?? {});
  if (run.llmCalls === 0 && models.length === 0) return null;

  return (
    <View style={styles.runUsage}>
      <View style={styles.runUsageHeader}>
        <Cpu size={13} color={colors.faint} />
        <AppText variant="caption" weight="semibold">Model usage</AppText>
        <AppText variant="caption" muted>
          {run.llmCalls} call{run.llmCalls === 1 ? '' : 's'} · {formatTokens(run.inputTokens)} in /{' '}
          {formatTokens(run.outputTokens)} out
        </AppText>
      </View>
      {models.map(([model, usage]) => (
        <View key={model} style={styles.usageModelRow}>
          <View style={styles.usageModelMain}>
            <AppText variant="caption" weight="medium" numberOfLines={1}>
              {model || 'unknown'}
            </AppText>
            <AppText variant="caption" tone="faint" numberOfLines={1}>
              {usage.provider ? `${usage.provider} · ` : ''}
              {usage.calls} call{usage.calls === 1 ? '' : 's'}
              {usage.failures ? ` · ${usage.failures} failed` : ''}
            </AppText>
          </View>
          <AppText variant="caption" muted tabular>
            {usage.inputTokens ? formatTokens(usage.inputTokens) : '—'} /{' '}
            {usage.outputTokens ? formatTokens(usage.outputTokens) : '—'}
            {usage.durationMs ? ` · ${formatDuration(usage.durationMs)}` : ''}
          </AppText>
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  chip: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.xs,
    borderRadius: radii.full
  },
  stepper: {
    flexDirection: 'row',
    alignItems: 'center'
  },
  stepItem: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs
  },
  connector: {
    flex: 1,
    height: 2,
    borderRadius: 1,
    marginRight: spacing.xs
  },
  dot: {
    width: 8,
    height: 8,
    borderRadius: 4
  },
  usageRow: {
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: spacing.xs
  },
  usageSummary: {
    flexShrink: 1
  },
  runUsage: {
    gap: spacing.sm,
    paddingTop: spacing.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  runUsageHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: spacing.xs
  },
  usageModelRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  usageModelMain: {
    flex: 1,
    gap: 1
  }
});
