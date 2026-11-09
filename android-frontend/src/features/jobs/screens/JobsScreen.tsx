import { useCallback, useEffect, useMemo, useState } from 'react';
import { Pressable, RefreshControl, StyleSheet, TextInput, View } from 'react-native';
import { ChevronDown, ChevronRight, ListChecks, X } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { EmptyState } from '../../../components/EmptyState';
import { Screen } from '../../../components/Screen';
import { StackHeader } from '../../../components/StackHeader';
import { colors, radii, spacing } from '../../../theme';
import { apiClient } from '../../../utils/api';
import { errorText } from '../../ai/llmSetup';
import { JobDetailSheet } from '../components/JobDetailSheet';
import { STATE_LABELS, formatAgo, formatTime, plural, rowStatus, rowTitle } from '../jobText';
import { buildRun, emailGroup, jobLabel, rowState, type EmailGroup, type JobRow, type JobRun } from '../jobTree';
import type { JobList } from '../types';

const PAGE_SIZE = 30;
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

type ShowFilter = 'all' | EmailGroup;

const SHOW_OPTIONS: { value: ShowFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'attention', label: 'Needs attention' },
  { value: 'active', label: 'In progress' },
  { value: 'created', label: 'Created' },
  { value: 'skipped', label: 'No new transaction' }
];

const GROUP_ORDER: EmailGroup[] = ['attention', 'active', 'created', 'skipped', 'cancelled'];

function runTitle(run: JobRun, attention: number, created: number): string {
  if (run.kind === 'other') return jobLabel(run.root.type);
  if (run.kind === 'message') return run.root.label || 'Message';
  const emails = plural(run.emailCount, 'email', 'emails');
  const isImport = run.root.type === 'mailbox.backfill';
  if (run.reading) return isImport ? 'Starting your import' : 'Syncing your mailbox';
  if (run.active) return `${isImport ? 'Importing' : 'Processing'} ${emails}`;
  if (run.emailCount === 0) return attention > 0 ? "Couldn't read the mailbox" : 'No new emails';
  if (attention > 0) return `${attention} of ${emails} need${attention === 1 ? 's' : ''} attention`;
  if (created > 0) return `${plural(created, 'transaction', 'transactions')} from ${emails}`;
  return `${emails}, no transactions`;
}

function statusTone(row: JobRow): 'danger' | 'primary' | 'muted' | 'success' {
  const group = emailGroup(row);
  if (group === 'attention') return 'danger';
  if (group === 'active') return 'primary';
  return group === 'created' ? 'success' : 'muted';
}

function RowItem({ row, divider, onPress }: { row: JobRow; divider: boolean; onPress: () => void }) {
  return (
    <Pressable
      accessibilityRole="button"
      style={({ pressed }) => [styles.row, divider && styles.divider, pressed && styles.pressed]}
      onPress={onPress}
    >
      <AppText numberOfLines={1} style={styles.flex}>{rowTitle(row)}</AppText>
      <AppText variant="caption" weight="medium" tone={statusTone(row)}>{rowStatus(row)}</AppText>
    </Pressable>
  );
}

function RunCard({
  run,
  show,
  query,
  forceOpen,
  onOpenRow
}: {
  run: JobRun;
  show: ShowFilter;
  query: string;
  forceOpen: boolean;
  onOpenRow: (row: JobRow) => void;
}) {
  const [userOpen, setUserOpen] = useState<boolean | null>(null);
  const needle = query.trim().toLowerCase();
  const rows = useMemo(
    () =>
      [...run.rows]
        .sort((a, b) => GROUP_ORDER.indexOf(emailGroup(a)) - GROUP_ORDER.indexOf(emailGroup(b)))
        .filter((row) => (show === 'all' || emailGroup(row) === show) && (!needle || rowTitle(row).toLowerCase().includes(needle))),
    [run.rows, show, needle]
  );
  const filtering = show !== 'all' || needle !== '';
  if (filtering && rows.length === 0) return null;

  const attention = run.rows.filter((row) => emailGroup(row) === 'attention').length;
  const created = run.rows.filter((row) => emailGroup(row) === 'created').length;

  // A single message goes straight to its details.
  if (run.kind !== 'mailbox' && rows.length === 1) {
    const row = rows[0];
    return (
      <Pressable accessibilityRole="button" style={({ pressed }) => pressed && styles.pressed} onPress={() => onOpenRow(row)}>
        <Card style={styles.runHead}>
          <View style={styles.flex}>
            <AppText weight="medium" numberOfLines={1}>{runTitle(run, attention, created)}</AppText>
            <AppText variant="caption" muted>{formatTime(run.root.createdAt)}</AppText>
          </View>
          <AppText variant="caption" weight="medium" tone={statusTone(row)}>
            {run.kind === 'other' ? STATE_LABELS[rowState(row)] : rowStatus(row)}
          </AppText>
        </Card>
      </Pressable>
    );
  }

  const open = userOpen ?? (run.active || attention > 0 || forceOpen || filtering);
  const Caret = open ? ChevronDown : ChevronRight;
  return (
    <Card>
      <Pressable accessibilityRole="button" accessibilityState={{ expanded: open }} style={styles.runHead} onPress={() => setUserOpen(!open)}>
        <View style={styles.flex}>
          <AppText weight="medium" numberOfLines={1}>{runTitle(run, attention, created)}</AppText>
          <AppText variant="caption" muted numberOfLines={1}>{run.root.label || 'Mailbox'} · {formatTime(run.root.createdAt)}</AppText>
        </View>
        <Caret size={16} color={colors.faint} />
      </Pressable>
      {open ? (
        <View style={styles.rows}>
          {rows.map((row, index) => (
            <RowItem key={row.jobs[0].id} row={row} divider={index > 0} onPress={() => onOpenRow(row)} />
          ))}
        </View>
      ) : null}
    </Card>
  );
}

/** The job queue for this budget, email by email. */
export function JobsScreen() {
  const [list, setList] = useState<JobList | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [errorCode, setErrorCode] = useState('');
  const [offset, setOffset] = useState(0);
  const [search, setSearch] = useState('');
  const [show, setShow] = useState<ShowFilter>('all');
  const [openRow, setOpenRow] = useState<JobRow | null>(null);

  const trimmed = search.trim();
  const jobId = UUID_PATTERN.test(trimmed) ? trimmed : '';

  const load = useCallback(async () => {
    const params = new URLSearchParams({ limit: String(PAGE_SIZE) });
    if (errorCode) params.set('errorCode', errorCode);
    if (jobId) params.set('jobId', jobId);
    if (offset) params.set('offset', String(offset));
    try {
      setList(await apiClient.get<JobList>(`jobs?${params.toString()}`));
      setError(null);
    } catch (err) {
      setError(errorText(err, 'Failed to load jobs'));
    }
  }, [errorCode, jobId, offset]);

  useEffect(() => {
    void load();
  }, [load]);

  // A failure that's been fixed loses its chip, and with it the way to turn its filter off.
  useEffect(() => {
    if (list && errorCode && !list.failures.some((failure) => failure.code === errorCode)) setErrorCode('');
  }, [list, errorCode]);

  const refresh = async () => {
    setRefreshing(true);
    await load();
    setRefreshing(false);
  };

  const filterByError = (code: string) => {
    setOffset(0);
    setErrorCode(code);
  };

  const allRuns = useMemo(() => (list?.trees ?? []).flatMap((tree) => buildRun(tree) ?? []), [list?.trees]);
  const isEmptySync = (run: JobRun) => run.kind === 'mailbox' && run.rows.length === 0 && !run.active;
  const runs = allRuns.filter((run) => !isEmptySync(run));
  const filtering = show !== 'all' || trimmed !== '' || errorCode !== '';
  const total = list?.total ?? 0;
  const failures = list?.failures ?? [];

  return (
    <Screen
      style={styles.screen}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={() => void refresh()} tintColor={colors.primary} />}
    >
      <StackHeader title="Jobs" subtitle="What happened to each message" />

      {list && !list.enabled ? (
        <EmptyState icon={<ListChecks color={colors.faint} size={24} />} title="The job queue is off" body="This server isn't running jobs." />
      ) : (
        <>
          <TextInput
            accessibilityLabel="Search by subject or job ID"
            placeholder="Search subject or paste a job ID"
            placeholderTextColor={colors.faint}
            value={search}
            autoCapitalize="none"
            autoCorrect={false}
            style={styles.search}
            onChangeText={(value) => {
              setOffset(0);
              setSearch(value);
            }}
          />

          <View style={styles.chips}>
            {SHOW_OPTIONS.map((option) => (
              <Pressable
                key={option.value}
                accessibilityRole="button"
                accessibilityState={{ selected: show === option.value }}
                style={[styles.chip, show === option.value && styles.chipOn]}
                onPress={() => setShow(option.value)}
              >
                <AppText variant="caption" weight="medium" tone={show === option.value ? 'primary' : 'muted'}>{option.label}</AppText>
              </Pressable>
            ))}
          </View>

          {failures.length > 0 ? (
            <View style={styles.failures}>
              <AppText variant="label" tone="faint">Failing now</AppText>
              {failures.map((failure) => {
                const on = errorCode === failure.code;
                return (
                  <Pressable
                    key={failure.code}
                    accessibilityRole="button"
                    accessibilityState={{ selected: on }}
                    style={[styles.failure, on && styles.failureOn]}
                    onPress={() => filterByError(on ? '' : failure.code)}
                  >
                    <AppText variant="caption" tone="danger" numberOfLines={1} style={styles.flex}>{failure.code}</AppText>
                    <AppText variant="caption" muted>{plural(failure.count, 'job', 'jobs')} · {formatAgo(failure.lastSeen)}</AppText>
                    {on ? <X size={14} color={colors.danger} /> : null}
                  </Pressable>
                );
              })}
            </View>
          ) : null}

          {errorCode ? (
            <View style={styles.filterNote}>
              <AppText variant="caption" muted style={styles.flex}>Only showing jobs failing with {errorCode}</AppText>
              <Pressable accessibilityRole="button" onPress={() => filterByError('')}>
                <AppText variant="caption" tone="primary" weight="medium">Show all jobs</AppText>
              </Pressable>
            </View>
          ) : null}

          {error ? (
            <Card>
              <AppText variant="caption" tone="danger">{error}</AppText>
            </Card>
          ) : !list ? (
            <Card>
              <AppText variant="caption" muted>Loading jobs…</AppText>
            </Card>
          ) : runs.length === 0 ? (
            <EmptyState icon={<ListChecks color={colors.faint} size={24} />} title={filtering ? 'Nothing matches' : 'No jobs yet'} />
          ) : null}

          {runs.map((run) => (
            <RunCard key={run.id} run={run} show={show} query={jobId ? '' : search} forceOpen={jobId !== '' || errorCode !== ''} onOpenRow={setOpenRow} />
          ))}

          {total > PAGE_SIZE ? (
            <View style={styles.pager}>
              <Button size="sm" variant="secondary" disabled={offset === 0} onPress={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>Newer</Button>
              <AppText variant="caption" muted>{offset + 1}–{Math.min(offset + PAGE_SIZE, total)} of {total}</AppText>
              <Button size="sm" variant="secondary" disabled={offset + PAGE_SIZE >= total} onPress={() => setOffset(offset + PAGE_SIZE)}>Older</Button>
            </View>
          ) : null}
        </>
      )}

      {openRow ? <JobDetailSheet key={openRow.jobs[0].id} row={openRow} onClose={() => setOpenRow(null)} onChanged={() => void load()} /> : null}
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: spacing.lg },
  flex: { flex: 1, minWidth: 0 },
  pressed: { opacity: 0.7 },
  search: {
    minHeight: 46,
    paddingHorizontal: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.surface,
    color: colors.text
  },
  chips: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  chip: { paddingHorizontal: spacing.md, paddingVertical: spacing.sm, borderRadius: radii.full, backgroundColor: colors.surface },
  chipOn: { backgroundColor: colors.primaryMuted },
  failures: { gap: spacing.sm },
  failure: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    padding: spacing.md,
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: 'transparent',
    backgroundColor: colors.surface
  },
  failureOn: { borderColor: colors.danger },
  filterNote: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  runHead: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  rows: { marginTop: spacing.sm },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.md, paddingVertical: spacing.md },
  divider: { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: colors.border },
  pager: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }
});
