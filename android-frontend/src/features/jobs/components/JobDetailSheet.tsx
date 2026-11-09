import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Sheet } from '../../../components/Sheet';
import { colors, radii, spacing } from '../../../theme';
import { apiClient } from '../../../utils/api';
import { errorText } from '../../ai/llmSetup';
import { formatDuration, rowReason, rowTitle } from '../jobText';
import { buildAttempts, focusJob, jobLabel, jobState, messageSteps, rowJobs, type JobRow, type MessageStep } from '../jobTree';
import type { JobDetail, MessageStepResult } from '../types';

function stepColor(state: MessageStep['state']): string {
  if (state === 'done') return colors.success;
  if (state === 'failed' || state === 'stopped') return colors.danger;
  if (state === 'running' || state === 'retrying') return colors.primary;
  return colors.surfaceTertiary;
}

function StepLine({ steps }: { steps: MessageStep[] }) {
  return (
    <View style={styles.track}>
      {steps.map((step) => (
        <View key={step.label} style={styles.trackStep}>
          <View style={[styles.node, { backgroundColor: stepColor(step.state) }]} />
          <AppText variant="caption" tone={step.state === 'pending' ? 'faint' : 'muted'} numberOfLines={1}>{step.label}</AppText>
        </View>
      ))}
    </View>
  );
}

function JsonSection({ title, value }: { title: string; value: unknown }) {
  const [open, setOpen] = useState(false);
  if (value === undefined || value === null) return null;
  return (
    <View style={styles.section}>
      <Pressable accessibilityRole="button" accessibilityState={{ expanded: open }} onPress={() => setOpen(!open)}>
        <AppText variant="caption" tone="primary" weight="medium">{open ? 'Hide' : 'Show'} {title.toLowerCase()}</AppText>
      </Pressable>
      {open ? (
        <View style={styles.json}>
          <AppText variant="caption" selectable style={styles.mono}>{JSON.stringify(value, null, 2)}</AppText>
        </View>
      ) : null}
    </View>
  );
}

export function JobDetailSheet({ row, onClose, onChanged }: { row: JobRow; onClose: () => void; onChanged: () => void }) {
  const jobs = rowJobs(row);
  const focus = focusJob(row);
  const [pickedId, setPickedId] = useState(focus.id);
  const [details, setDetails] = useState<Record<string, JobDetail>>({});
  const [technical, setTechnical] = useState(false);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);

  const load = useCallback(async (id: string) => {
    try {
      const detail = await apiClient.get<JobDetail>(`jobs/${id}`);
      setDetails((current) => ({ ...current, [id]: detail }));
    } catch {
      setFailed(true);
    }
  }, []);

  useEffect(() => {
    void load(focus.id);
  }, [focus.id, load]);

  useEffect(() => {
    void load(pickedId);
  }, [pickedId, load]);

  const shown = details[pickedId];
  const attempts = useMemo(() => (shown ? buildAttempts(shown.events) : []), [shown]);
  const stepResult = shown?.result as MessageStepResult | null | undefined;
  const dead = jobs.find((job) => job.status === 'dead');
  const reason = (details[focus.id]?.payload as { reason?: string } | undefined)?.reason;
  const isMessage = row.jobs[0].type === 'message.received';

  const act = async (id: string, action: 'retry' | 'cancel') => {
    setBusy(true);
    try {
      await apiClient.post(`jobs/${id}/${action}`, {});
      onChanged();
      onClose();
    } catch (err) {
      Alert.alert(`Couldn't ${action} the job`, errorText(err, 'Please try again.'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Sheet
      visible
      size="tall"
      title={rowTitle(row)}
      onClose={onClose}
      footer={dead ? <Button disabled={busy} onPress={() => void act(dead.id, 'retry')}>{busy ? 'Queueing…' : 'Retry'}</Button> : undefined}
    >
      <ScrollView contentContainerStyle={styles.body}>
        {isMessage ? <StepLine steps={messageSteps(row)} /> : null}

        <View style={styles.section}>
          <AppText variant="label" tone="faint">What happened</AppText>
          <AppText>{rowReason(row, reason)}</AppText>
          {['failed', 'retrying'].includes(jobState(focus)) && focus.lastError ? (
            <AppText variant="caption" tone="danger" selectable>{focus.lastError}</AppText>
          ) : null}
          {!details[focus.id] && failed ? <AppText variant="caption" muted>Couldn't load the details of this job.</AppText> : null}
        </View>

        <Pressable accessibilityRole="button" accessibilityState={{ expanded: technical }} onPress={() => setTechnical(!technical)}>
          <AppText variant="caption" tone="primary" weight="medium">{technical ? 'Hide' : 'Show'} technical details</AppText>
        </Pressable>

        {technical ? (
          <View style={styles.technical}>
            {jobs.length > 1 ? (
              <View style={styles.picker}>
                {jobs.map((job) => (
                  <Pressable
                    key={job.id}
                    accessibilityRole="button"
                    accessibilityState={{ selected: job.id === pickedId }}
                    style={[styles.pick, job.id === pickedId && styles.pickOn]}
                    onPress={() => setPickedId(job.id)}
                  >
                    <AppText variant="caption" weight="medium" tone={job.id === pickedId ? 'primary' : 'muted'}>{jobLabel(job.type)}</AppText>
                  </Pressable>
                ))}
              </View>
            ) : null}
            {shown ? (
              <>
                {attempts.map((attempt, index) => (
                  <View key={attempt.startedAt} style={styles.section}>
                    <AppText variant="caption">
                      Attempt {index + 1}
                      {attempt.durationMs !== null ? ` · ${formatDuration(attempt.durationMs)}` : ''}
                      {attempt.end && attempt.end.type !== 'completed' ? ' · Failed' : ''}
                    </AppText>
                    {attempt.end?.error ? <AppText variant="caption" tone="danger" selectable>{attempt.end.error}</AppText> : null}
                  </View>
                ))}
                <JsonSection title="Input" value={shown.payload} />
                <JsonSection title="Output" value={stepResult?.output ?? shown.result} />
                {stepResult?.nextJobType ? (
                  <AppText variant="caption" muted>Next step: {jobLabel(stepResult.nextJobType)}</AppText>
                ) : null}
                <AppText variant="caption" tone="faint" selectable>{shown.type} on {shown.queue} · {shown.id}</AppText>
                {shown.status === 'queued' ? (
                  <Button size="sm" variant="secondary" disabled={busy} onPress={() => void act(shown.id, 'cancel')}>Cancel job</Button>
                ) : null}
              </>
            ) : (
              <AppText variant="caption" muted>Loading…</AppText>
            )}
          </View>
        ) : null}
      </ScrollView>
    </Sheet>
  );
}

const styles = StyleSheet.create({
  body: { gap: spacing.lg },
  section: { gap: spacing.xs },
  track: { flexDirection: 'row', gap: 2 },
  trackStep: { flex: 1, minWidth: 0, alignItems: 'center', gap: spacing.xs },
  node: { width: 12, height: 12, borderRadius: 6 },
  technical: { gap: spacing.md },
  picker: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.xs },
  pick: { paddingHorizontal: spacing.md, paddingVertical: spacing.xs, borderRadius: radii.full, backgroundColor: colors.surfaceStrong },
  pickOn: { backgroundColor: colors.primaryMuted },
  json: { padding: spacing.md, borderRadius: radii.md, backgroundColor: colors.surfaceStrong },
  mono: { fontFamily: 'monospace' }
});
