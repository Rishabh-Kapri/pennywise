import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Pressable, RefreshControl, StyleSheet, TextInput, View } from 'react-native';
import { Ban, ChevronDown, ChevronRight, CircleCheck, Inbox, ListChecks, LoaderCircle, TriangleAlert, X } from 'lucide-react-native';
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
import { STATE_LABELS, formatAgo, plural, rowStatus, rowTitle } from '../jobText';
import { buildRun, emailGroup, jobLabel, rowState, type EmailGroup, type JobRow, type JobRun } from '../jobTree';
import { mergeBySource, splitByDay, type ListedRun } from '../runGroups';
import type { JobList } from '../types';

const PAGE_SIZE = 30;
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

type ShowFilter = 'all' | EmailGroup;
type Tone = 'danger' | 'primary' | 'muted' | 'success';

const SHOW_OPTIONS: { value: ShowFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'attention', label: 'Needs attention' },
  { value: 'active', label: 'In progress' },
  { value: 'created', label: 'Created' },
  { value: 'skipped', label: 'No new transaction' }
];

const GROUP_TONES: Record<EmailGroup, Tone> = {
  attention: 'danger',
  active: 'primary',
  created: 'success',
  skipped: 'muted',
  cancelled: 'muted'
};

const TONE_COLORS: Record<Tone, string> = {
  danger: colors.danger,
  primary: colors.primary,
  success: colors.success,
  muted: colors.faint
};

// Same order and defaults as the web: the outcomes first, the rest folded away.
const SECTIONS: { group: EmailGroup; title: string; Icon: typeof Inbox; openByDefault: boolean }[] = [
  { group: 'attention', title: 'Needs attention', Icon: TriangleAlert, openByDefault: true },
  { group: 'created', title: 'Transactions created', Icon: CircleCheck, openByDefault: true },
  { group: 'active', title: 'In progress', Icon: LoaderCircle, openByDefault: false },
  { group: 'skipped', title: 'No new transaction', Icon: Inbox, openByDefault: false },
  { group: 'cancelled', title: 'Cancelled', Icon: Ban, openByDefault: false }
];

function formatHourMinute(date: string): string {
  return new Date(date).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

function dayLabel(date: string): string {
  const day = new Date(date);
  const yesterday = new Date();
  yesterday.setDate(yesterday.getDate() - 1);
  if (day.toDateString() === new Date().toDateString()) return 'Today';
  if (day.toDateString() === yesterday.toDateString()) return 'Yesterday';
  return day.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' });
}

/** What became of a finished batch. `items` is already counted, like "3 emails". */
function outcomeTitle(items: string, attention: number, created: number): string {
  if (attention > 0) return `${attention} of ${items} need${attention === 1 ? 's' : ''} attention`;
  if (created > 0) return `${plural(created, 'transaction', 'transactions')} from ${items}`;
  return `${items}, no transactions`;
}

function runTitle(run: ListedRun, attention: number, created: number): string {
  if (run.kind === 'other') return jobLabel(run.root.type);
  if (run.kind === 'message') {
    if (!run.startedAt) return run.root.label || 'Message';
    const messages = plural(run.emailCount, 'message', 'messages');
    return run.active ? `Processing ${messages}` : outcomeTitle(messages, attention, created);
  }
  const emails = plural(run.emailCount, 'email', 'emails');
  const isImport = run.root.type === 'mailbox.backfill';
  if (run.reading) return isImport ? 'Starting your import' : 'Syncing your mailbox';
  if (run.active) return `${isImport ? 'Importing' : 'Processing'} ${emails}`;
  if (run.emailCount === 0) return attention > 0 ? "Couldn't read the mailbox" : 'No new emails';
  if (isImport && attention === 0 && created === 0) return `${emails} imported, no transactions`;
  return outcomeTitle(emails, attention, created);
}

/** Source and time. The day is in the heading above the card. */
function runMeta(run: ListedRun): string {
  const spansTime = run.startedAt && formatHourMinute(run.startedAt) !== formatHourMinute(run.root.createdAt);
  const time = spansTime
    ? `${formatHourMinute(run.startedAt!)} – ${formatHourMinute(run.root.createdAt)}`
    : formatHourMinute(run.root.createdAt);
  if (run.kind === 'mailbox') return `${run.root.label || 'Mailbox'} · ${time}`;
  if (run.kind === 'message') {
    const source = { sms: 'SMS', api: 'API', email: 'Email' }[run.root.source ?? ''] ?? 'Message';
    return `${source} · ${time}`;
  }
  return `${STATE_LABELS[rowState({ jobs: [run.root], children: [] })]} · ${time}`;
}

function RowItem({ row, divider, onPress }: { row: JobRow; divider: boolean; onPress: () => void }) {
  return (
    <Pressable
      accessibilityRole="button"
      style={({ pressed }) => [styles.row, divider && styles.divider, pressed && styles.pressed]}
      onPress={onPress}
    >
      <AppText numberOfLines={1} style={styles.flex}>{rowTitle(row)}</AppText>
      <AppText variant="caption" weight="medium" tone={GROUP_TONES[emailGroup(row)]}>{rowStatus(row)}</AppText>
    </Pressable>
  );
}

function Section({
  group,
  title,
  icon,
  rows,
  defaultOpen,
  onOpenRow
}: {
  group: EmailGroup;
  title: string;
  icon: ReactNode;
  rows: JobRow[];
  defaultOpen: boolean;
  onOpenRow: (row: JobRow) => void;
}) {
  const [userOpen, setUserOpen] = useState<boolean | null>(null);
  const open = userOpen ?? defaultOpen;
  const Caret = open ? ChevronDown : ChevronRight;
  return (
    <View style={styles.section}>
      <Pressable
        accessibilityRole="button"
        accessibilityState={{ expanded: open }}
        style={({ pressed }) => [styles.sectionHead, pressed && styles.pressed]}
        onPress={() => setUserOpen(!open)}
      >
        {icon}
        <AppText variant="caption" weight="medium" tone={GROUP_TONES[group]} style={styles.flex}>{title}</AppText>
        <AppText variant="caption" muted tabular>{rows.length}</AppText>
        <Caret size={14} color={colors.faint} />
      </Pressable>
      {open
        ? rows.map((row, index) => <RowItem key={row.jobs[0].id} row={row} divider={index > 0} onPress={() => onOpenRow(row)} />)
        : null}
    </View>
  );
}

function RunCard({
  run,
  show,
  query,
  forceOpen,
  onOpenRow
}: {
  run: ListedRun;
  show: ShowFilter;
  query: string;
  forceOpen: boolean;
  onOpenRow: (row: JobRow) => void;
}) {
  const [userOpen, setUserOpen] = useState<boolean | null>(null);
  const groups = useMemo(() => {
    const byGroup: Record<EmailGroup, JobRow[]> = { attention: [], active: [], created: [], skipped: [], cancelled: [] };
    for (const row of run.rows) byGroup[emailGroup(row)].push(row);
    return byGroup;
  }, [run.rows]);

  const needle = query.trim().toLowerCase();
  const visible = (group: EmailGroup): JobRow[] => {
    if (show !== 'all' && show !== group) return [];
    if (!needle) return groups[group];
    return groups[group].filter((row) => rowTitle(row).toLowerCase().includes(needle));
  };
  const shownSections = SECTIONS.filter(({ group }) => visible(group).length > 0);
  const filtering = show !== 'all' || needle !== '';
  if (filtering && shownSections.length === 0) return null;

  const attention = groups.attention.length;
  const created = groups.created.length;
  const title = runTitle(run, attention, created);

  // A single message goes straight to its details.
  if (run.kind !== 'mailbox' && !run.startedAt && run.rows.length === 1) {
    const row = run.rows[0];
    return (
      <Pressable accessibilityRole="button" style={({ pressed }) => pressed && styles.pressed} onPress={() => onOpenRow(row)}>
        <Card style={styles.runHead}>
          <View style={styles.flex}>
            <AppText weight="medium" numberOfLines={1}>{title}</AppText>
            <AppText variant="caption" muted numberOfLines={1}>{runMeta(run)}</AppText>
          </View>
          <AppText variant="caption" weight="medium" tone={GROUP_TONES[emailGroup(row)]}>
            {run.kind === 'other' ? STATE_LABELS[rowState(row)] : rowStatus(row)}
          </AppText>
        </Card>
      </Pressable>
    );
  }

  // A day's card can hold many emails; opened, it would push the other days off screen.
  const flagged = attention > 0 && !run.startedAt;
  const open = userOpen ?? (run.active || flagged || forceOpen || filtering);
  const Caret = open ? ChevronDown : ChevronRight;
  const Icon = run.active ? LoaderCircle : attention > 0 ? TriangleAlert : null;
  return (
    <Card>
      <Pressable accessibilityRole="button" accessibilityState={{ expanded: open }} style={styles.runHead} onPress={() => setUserOpen(!open)}>
        {Icon ? <Icon size={18} color={run.active ? colors.primary : colors.danger} /> : null}
        <View style={styles.flex}>
          <AppText weight="medium" numberOfLines={1}>{title}</AppText>
          <AppText variant="caption" muted numberOfLines={1}>{runMeta(run)}</AppText>
        </View>
        <Caret size={16} color={colors.faint} />
      </Pressable>
      {open ? (
        <View style={styles.sections}>
          {shownSections.map(({ group, title: sectionTitle, Icon: SectionIcon, openByDefault }) => (
            <Section
              key={group}
              group={group}
              title={sectionTitle}
              icon={<SectionIcon size={14} color={TONE_COLORS[GROUP_TONES[group]]} />}
              rows={visible(group)}
              defaultOpen={openByDefault || filtering}
              onOpenRow={onOpenRow}
            />
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
  const filtering = show !== 'all' || trimmed !== '' || errorCode !== '';
  // Filters look for particular emails, so matches stay under their own run.
  const runs = useMemo(() => {
    const shown = allRuns.filter((run) => !isEmptySync(run));
    return filtering ? shown : mergeBySource(shown);
  }, [allRuns, filtering]);
  const days = useMemo(() => splitByDay(runs), [runs]);
  const anyAttention = runs.some((run) => run.rows.some((row) => emailGroup(row) === 'attention'));
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
          ) : !anyAttention && !filtering && offset === 0 ? (
            <View style={styles.allClear}>
              <CircleCheck size={16} color={colors.success} />
              <AppText variant="caption" tone="success">Nothing needs attention.</AppText>
            </View>
          ) : null}

          {days.map((day) => (
            <View key={day.key} style={styles.day}>
              <AppText variant="label" tone="faint">{dayLabel(day.date)}</AppText>
              {day.runs.map((run) => (
                <RunCard key={run.id} run={run} show={show} query={jobId ? '' : search} forceOpen={jobId !== '' || errorCode !== ''} onOpenRow={setOpenRow} />
              ))}
            </View>
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
  allClear: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  day: { gap: spacing.sm },
  sections: { marginTop: spacing.sm, gap: spacing.xs },
  section: { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: colors.border },
  sectionHead: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, paddingVertical: spacing.md },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.md, paddingVertical: spacing.md, paddingLeft: spacing.xl },
  divider: { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: colors.border },
  pager: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }
});
