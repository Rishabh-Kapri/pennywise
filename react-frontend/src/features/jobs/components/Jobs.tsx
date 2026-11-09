import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useHeader } from '@/context/HeaderContext';
import { selectIsDemoUser } from '@/features/auth/store/authSlice';
import { LoadingState, errorMessage, toast } from '@/utils';
import {
  ArrowsClockwise,
  CaretDown,
  CaretRight,
  CheckCircle,
  CircleNotch,
  Copy,
  EnvelopeSimple,
  MagnifyingGlass,
  Prohibit,
  Tray,
  Warning,
  X,
} from '@phosphor-icons/react';
import { AnimatePresence, MotionConfig, motion } from 'framer-motion';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import {
  JOBS_PAGE_SIZE,
  fetchJobDetail,
  fetchJobs,
  jobDetailsCleared,
  jobFiltersChanged,
  retryFailedJobs,
  runJobAction,
} from '../store';
import {
  buildAttempts,
  buildRun,
  emailGroup,
  focusJob,
  isMailboxJob,
  isStopped,
  jobLabel,
  jobState,
  lastStepLabel,
  messageSteps,
  rowJobs,
  rowOutcome,
  rowState,
  type EmailGroup,
  type JobRow,
  type JobRun,
  type JobState,
  type MessageStep,
} from '../jobTree';
import type { JobDetail, JobSummary, MessageStepResult } from '../types/job.types';
import styles from './Jobs.module.css';
import { PipelineView } from './PipelineView';
import { messagePrefill } from '../messagePrefill';

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const STATE_LABELS: Record<JobState, string> = {
  done: 'Done',
  running: 'Running',
  queued: 'Queued',
  retrying: 'Retrying',
  failed: 'Failed',
  cancelled: 'Cancelled',
};

const OUTCOME_LABELS: Record<string, string> = {
  not_transaction: 'Not a transaction',
  unverified: 'Not verified',
  no_prediction: 'No category found',
  transaction_created: 'Transaction created',
  transaction_duplicate: 'Already added',
};

const OUTCOME_REASONS: Record<string, string> = {
  not_transaction: "The check decided this email isn't a transaction.",
  unverified: "Marked as a transaction, but it couldn't be verified.",
  no_prediction: "Couldn't work out a payee and category for it.",
  transaction_created: 'A transaction was created from this email.',
  transaction_duplicate: 'This matches a transaction that already exists, so nothing new was added.',
};

type ShowFilter = 'all' | 'attention' | 'active' | 'created' | 'skipped';

const SHOW_OPTIONS: { value: ShowFilter; label: string }[] = [
  { value: 'all', label: 'All emails' },
  { value: 'attention', label: 'Needs attention' },
  { value: 'active', label: 'In progress' },
  { value: 'created', label: 'Transactions created' },
  { value: 'skipped', label: 'No new transaction' },
];

const SECTIONS: { group: EmailGroup; title: string; icon: ReactNode; openByDefault: boolean }[] = [
  { group: 'attention', title: 'Needs attention', icon: <Warning size={16} weight="fill" />, openByDefault: true },
  {
    group: 'created',
    title: 'Transactions created',
    icon: <CheckCircle size={16} weight="fill" />,
    openByDefault: true,
  },
  { group: 'active', title: 'In progress', icon: <CircleNotch size={16} />, openByDefault: false },
  { group: 'skipped', title: 'No new transaction', icon: <Tray size={16} />, openByDefault: false },
  { group: 'cancelled', title: 'Cancelled', icon: <Prohibit size={16} />, openByDefault: false },
];

const EASE_OUT = [0.22, 1, 0.36, 1] as const;

function formatTime(dateString: string): string {
  return new Date(dateString).toLocaleString(undefined, {
    day: 'numeric',
    month: 'short',
    hour: 'numeric',
    minute: '2-digit',
  });
}

function formatClock(dateString: string): string {
  return new Date(dateString).toLocaleTimeString(undefined, {
    hour: 'numeric',
    minute: '2-digit',
    second: '2-digit',
  });
}

function formatAgo(dateString: string): string {
  const minutes = Math.floor((Date.now() - new Date(dateString).getTime()) / 60_000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60_000)}m ${Math.round((ms % 60_000) / 1000)}s`;
}

function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`;
}

function rowTitle(row: JobRow): string {
  const first = row.jobs[0];
  if (first.type === 'mailbox.backfill') return 'Reading the chosen emails';
  if (first.type === 'mailbox.changed') return 'Reading the mailbox';
  return first.label || jobLabel(first.type);
}

/** The short status on the right of a row. */
function rowStatus(row: JobRow): string {
  const state = rowState(row);
  if (state !== 'done') return STATE_LABELS[state];
  const outcome = rowOutcome(row);
  return outcome ? (OUTCOME_LABELS[outcome] ?? outcome) : STATE_LABELS.done;
}

/** Only shown on rows that need attention; the section explains the others. */
function rowSubtitle(row: JobRow): string {
  const verbs: Partial<Record<JobState, string>> = { failed: 'Failed at', retrying: 'Retrying' };
  return `${verbs[rowState(row)] ?? 'Stopped at'} ${lastStepLabel(row)}`;
}

/** Why a row ended up where it did, in a sentence. */
function rowReason(row: JobRow, reason?: string): string {
  const job = focusJob(row);
  const state = jobState(job);
  const step = lastStepLabel(row);
  if (state === 'failed') return `${step} failed after ${plural(job.attempt, 'attempt', 'attempts')}.`;
  if (state === 'retrying') {
    return `${step} failed. Trying again, attempt ${job.attempt + 1} of ${job.maxAttempts}.`;
  }
  const outcome = rowOutcome(row) ?? '';
  return reason || OUTCOME_REASONS[outcome] || STATE_LABELS[rowState(row)];
}

/** The email a pipeline job carries: on its own, or under `message`. */
function jobMessage(detail?: JobDetail): { sender?: string; receivedAt?: string; reason?: string } {
  if (!detail?.payload || typeof detail.payload !== 'object') return {};
  const payload = detail.payload as {
    message?: { sender?: string; receivedAt?: string };
    sender?: string;
    receivedAt?: string;
    reason?: string;
  };
  const message = payload.message ?? payload;
  return { sender: message.sender, receivedAt: message.receivedAt, reason: payload.reason };
}

/** One stroke of a step icon, drawn in when the step reaches this state. */
function DrawnPath({ d, delay }: { d: string; delay: number }) {
  return (
    <motion.path
      d={d}
      fill="none"
      stroke="currentColor"
      strokeWidth={2.5}
      strokeLinecap="round"
      strokeLinejoin="round"
      initial={{ pathLength: 0 }}
      animate={{ pathLength: 1 }}
      transition={{ duration: 0.3, delay, ease: 'easeOut' }}
    />
  );
}

function StepNode({ state, index }: { state: MessageStep['state']; index: number }) {
  const delay = index * 0.07;
  const settled = ['done', 'failed', 'stopped', 'skipped'].includes(state);
  return (
    <motion.span
      key={state}
      className={styles.node}
      initial={settled ? { scale: 0.5, opacity: 0 } : false}
      animate={{ scale: 1, opacity: 1 }}
      transition={{ type: 'spring', stiffness: 420, damping: 22, delay }}
    >
      {settled && (
        <svg viewBox="0 0 16 16" width={12} height={12} aria-hidden>
          {state === 'done' && <DrawnPath d="M3.5 8.5 6.8 11.5 12.5 4.8" delay={delay + 0.08} />}
          {state === 'skipped' && <DrawnPath d="M4 8H12" delay={delay + 0.08} />}
          {(state === 'failed' || state === 'stopped') && (
            <>
              <DrawnPath d="M4.5 4.5 11.5 11.5" delay={delay + 0.08} />
              <DrawnPath d="M11.5 4.5 4.5 11.5" delay={delay + 0.2} />
            </>
          )}
        </svg>
      )}
    </motion.span>
  );
}

/** The pipeline as one track: it fills to the current step, and a light runs on from there. */
function StepLine({ steps }: { steps: MessageStep[] }) {
  const reached = steps.reduce((last, step, index) => (['pending', 'queued'].includes(step.state) ? last : index), 0);
  const working = steps.findIndex((step) => step.state === 'running' || step.state === 'retrying');
  const hasStopped = steps.some((step) => step.state === 'failed' || step.state === 'stopped');
  const share = 100 / steps.length;

  return (
    <ol className={styles.track} style={{ gridTemplateColumns: `repeat(${steps.length}, minmax(0, 1fr))` }}>
      <li aria-hidden className={styles.rail} style={{ left: `${share / 2}%`, right: `${share / 2}%` }}>
        <motion.span
          className={`${styles.railFill} ${hasStopped ? styles.railFillStopped : ''}`}
          initial={{ scaleX: 0 }}
          animate={{ scaleX: reached / (steps.length - 1) }}
          transition={{ duration: 0.5, ease: EASE_OUT }}
        />
      </li>
      {working >= 0 && working < steps.length - 1 && (
        <li aria-hidden className={styles.comet} style={{ left: `${share * (working + 0.5)}%`, width: `${share}%` }} />
      )}
      {steps.map((step, index) => (
        <li key={step.label} className={`${styles.step} ${styles[`step_${step.state}`]}`}>
          <StepNode state={step.state} index={index} />
          <span className={styles.stepLabel}>{step.label}</span>
        </li>
      ))}
    </ol>
  );
}

function JsonSection({ title, value }: { title: string; value: unknown }) {
  const [open, setOpen] = useState(false);
  if (value === undefined || value === null) return null;
  return (
    <div>
      <button type="button" className={styles.textToggle} aria-expanded={open} onClick={() => setOpen(!open)}>
        {open ? <CaretDown size={14} /> : <CaretRight size={14} />}
        {title}
      </button>
      {open && <pre className={styles.json}>{JSON.stringify(value, null, 2)}</pre>}
    </div>
  );
}

function JobTechnical({ detail }: { detail: JobDetail }) {
  const dispatch = useAppDispatch();
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const busy = useAppSelector((state) => state.jobs.actingJobId === detail.id);
  const attempts = useMemo(() => buildAttempts(detail.events), [detail.events]);
  const stepResult = detail.result as MessageStepResult | null | undefined;

  const cancel = async () => {
    try {
      await dispatch(runJobAction({ id: detail.id, action: 'cancel' })).unwrap();
      toast.success('Job cancelled');
    } catch (err) {
      toast.error("Couldn't cancel job", { description: errorMessage(err, 'Please try again.') });
    }
  };

  const copyId = async () => {
    try {
      await navigator.clipboard.writeText(detail.id);
      toast.success('Job ID copied');
    } catch {
      toast.error("Couldn't copy the job ID");
    }
  };

  return (
    <div className={styles.technicalBody}>
      <div className={styles.attempts}>
        {attempts.length === 0 && <span className={styles.muted}>Not started yet.</span>}
        {attempts.map((attempt, index) => (
          <div key={attempt.startedAt} className={styles.attempt}>
            <span className={styles.attemptLine}>
              Attempt {index + 1}
              <span className={styles.muted}>
                {formatClock(attempt.startedAt)}
                {attempt.durationMs !== null && ` · ${formatDuration(attempt.durationMs)}`}
              </span>
              {attempt.end && attempt.end.type !== 'completed' && <span className={styles.bad}>Failed</span>}
            </span>
            {attempt.end?.error && <p className={styles.errorText}>{attempt.end.error}</p>}
          </div>
        ))}
      </div>

      <JsonSection title="Input" value={detail.payload} />
      <JsonSection title="Output" value={stepResult?.output ?? detail.result} />

      <dl className={styles.facts}>
        <dt>Job</dt>
        <dd>
          {detail.type} on {detail.queue}
        </dd>
        {stepResult?.nextJobType && (
          <>
            <dt>Next step</dt>
            <dd>{jobLabel(stepResult.nextJobType)}</dd>
          </>
        )}
        {detail.lockedBy && (
          <>
            <dt>Worker</dt>
            <dd>{detail.lockedBy}</dd>
          </>
        )}
        {detail.originService && (
          <>
            <dt>Started by</dt>
            <dd>{detail.originService}</dd>
          </>
        )}
        {detail.correlationId && (
          <>
            <dt>Correlation ID</dt>
            <dd>{detail.correlationId}</dd>
          </>
        )}
      </dl>

      <div className={styles.actions}>
        {!isDemoUser && detail.status === 'queued' && (
          <button type="button" className={styles.button} disabled={busy} onClick={cancel}>
            <Prohibit size={16} />
            Cancel job
          </button>
        )}
        <button type="button" className={styles.button} onClick={copyId}>
          <Copy size={16} />
          Copy job ID
        </button>
      </div>
    </div>
  );
}

/** What opening a row shows. It loads the jobs it needs, again after each refresh. */
function RowDetails({ row }: { row: JobRow }) {
  const dispatch = useAppDispatch();
  const details = useAppSelector((state) => state.jobs.details);
  const detailLoading = useAppSelector((state) => state.jobs.detailLoading);
  const updatedAt = useAppSelector((state) => state.jobs.updatedAt);
  const [technicalOpen, setTechnicalOpen] = useState(false);
  const [pickedId, setPickedId] = useState<string | null>(null);

  const focus = focusJob(row);
  const jobs = rowJobs(row);
  const shownId = pickedId && jobs.some((job) => job.id === pickedId) ? pickedId : focus.id;
  const summary = details[focus.id];
  const shown = details[shownId];
  const isMessage = row.jobs[0].type === 'message.received';
  const { sender, receivedAt, reason } = jobMessage(summary);

  useEffect(() => {
    dispatch(fetchJobDetail(focus.id));
    if (shownId !== focus.id) dispatch(fetchJobDetail(shownId));
  }, [dispatch, focus.id, shownId, updatedAt]);

  return (
    <div className={styles.rowDetails}>
      {isMessage && <StepLine steps={messageSteps(row)} />}

      <div>
        <h4 className={styles.detailHeading}>What happened</h4>
        <p>{rowReason(row, reason)}</p>
        {/* A job that recovered keeps its last error; that's history, in technical details. */}
        {['failed', 'retrying'].includes(jobState(focus)) && focus.lastError && (
          <p className={styles.errorText}>{focus.lastError}</p>
        )}
      </div>

      {sender && (
        <p className={styles.muted}>
          From {sender}
          {receivedAt && ` · Received ${formatTime(receivedAt)}`}
        </p>
      )}
      {!summary && detailLoading === LoadingState.ERROR && (
        <p className={styles.muted}>Couldn't load the details of this job.</p>
      )}

      <div className={styles.actions}>
        <button
          type="button"
          className={styles.textToggle}
          aria-expanded={technicalOpen}
          onClick={() => setTechnicalOpen(!technicalOpen)}
        >
          {technicalOpen ? <CaretDown size={14} /> : <CaretRight size={14} />}
          Technical details
        </button>
      </div>

      {technicalOpen && (
        <div className={styles.technical}>
          {jobs.length > 1 && (
            <div className={styles.jobPicker} role="group" aria-label="Pipeline jobs">
              {jobs.map((job) => (
                <button
                  key={job.id}
                  type="button"
                  aria-pressed={job.id === shownId}
                  className={`${styles.jobPick} ${job.id === shownId ? styles.jobPickOn : ''}`}
                  onClick={() => setPickedId(job.id)}
                >
                  {jobLabel(job.type)}
                </button>
              ))}
            </div>
          )}
          {shown ? <JobTechnical detail={shown} /> : <p className={styles.muted}>Loading…</p>}
        </div>
      )}
    </div>
  );
}

/** What a person can do about a row that needs attention, without opening it. */
function RowActions({ row }: { row: JobRow }) {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const actingJobId = useAppSelector((state) => state.jobs.actingJobId);
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const payees = useAppSelector((state) => state.payees.allPayees);
  const [preparing, setPreparing] = useState(false);

  const first = row.jobs[0];
  const dead = rowJobs(row).find((job: JobSummary) => job.status === 'dead');
  const stopped = isStopped(row);
  // Still retrying on its own, so there's nothing to decide yet.
  if (isDemoUser || (!dead && !stopped)) return null;

  const isMessage = first.type === 'message.received';
  const busy = preparing || actingJobId !== null;

  const act = async (id: string, action: 'retry' | 'run-again' | 'dismiss', done: string, failed: string) => {
    try {
      await dispatch(runJobAction({ id, action })).unwrap();
      toast.success(done);
    } catch (err) {
      toast.error(failed, { description: errorMessage(err, 'Please try again.') });
    }
  };

  const dismiss = () => {
    if (!window.confirm("Dismiss this email? It won't come back here, and importing it again won't bring it back."))
      return;
    void act(first.id, 'dismiss', 'Dismissed', "Couldn't dismiss it");
  };

  const createByHand = async () => {
    // The latest step of the message carries everything extracted so far.
    const source = [...row.jobs].reverse().find((job) => job.type !== 'pipeline.report') ?? first;
    setPreparing(true);
    try {
      const detail = await dispatch(fetchJobDetail(source.id)).unwrap();
      navigate('/transactions', {
        state: { prefill: messagePrefill(detail, accounts, payees), dismissJobId: first.id },
      });
    } catch (err) {
      toast.error("Couldn't read this email's details", { description: errorMessage(err, 'Please try again.') });
      setPreparing(false);
    }
  };

  return (
    <div className={styles.rowActions}>
      {dead && (
        <button
          type="button"
          className={styles.primaryButton}
          disabled={busy}
          onClick={() => void act(dead.id, 'retry', 'Queued again', "Couldn't retry it")}
        >
          <ArrowsClockwise size={16} className={actingJobId === dead.id ? styles.spin : undefined} />
          Retry
        </button>
      )}
      {stopped && (
        <button
          type="button"
          className={styles.primaryButton}
          disabled={busy}
          onClick={() => void act(first.id, 'run-again', 'Running it again', "Couldn't run it again")}
        >
          <ArrowsClockwise size={16} className={actingJobId === first.id ? styles.spin : undefined} />
          Run again
        </button>
      )}
      {isMessage && (
        <button type="button" className={styles.button} disabled={busy} onClick={() => void createByHand()}>
          Create by hand
        </button>
      )}
      {!isMailboxJob(first.type) && (
        <button type="button" className={styles.textToggle} disabled={busy} onClick={dismiss}>
          Dismiss
        </button>
      )}
    </div>
  );
}

function EmailRow({ row, open, onToggle }: { row: JobRow; open: boolean; onToggle: () => void }) {
  const group = emailGroup(row);
  const full = group === 'attention';
  const statusClass = rowState(row) === 'failed' ? 'failed' : group;
  return (
    <motion.div
      layout="position"
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.25, ease: EASE_OUT }}
      className={`${styles.email} ${open ? styles.emailOpen : ''}`}
    >
      <button type="button" className={styles.emailRow} aria-expanded={open} onClick={onToggle}>
        <span className={styles.emailText}>
          <span className={styles.emailTitle}>{rowTitle(row)}</span>
          {full && <span className={styles.emailMeta}>{rowSubtitle(row)}</span>}
        </span>
        {(full || group === 'active') && (
          <span className={`${styles.status} ${styles[`status_${statusClass}`]}`}>{rowStatus(row)}</span>
        )}
        <CaretRight size={16} className={`${styles.caret} ${open ? styles.caretOpen : ''}`} />
      </button>
      {full && <RowActions row={row} />}
      {open && <RowDetails row={row} />}
    </motion.div>
  );
}

interface RowListProps {
  openRowId: string | null;
  onToggle: (id: string) => void;
}

/** A number that rolls up when it changes. */
function RollingCount({ value }: { value: number }) {
  return (
    <span className={styles.roll}>
      <motion.span
        key={value}
        initial={{ y: '70%', opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        transition={{ duration: 0.3, ease: EASE_OUT }}
      >
        {value}
      </motion.span>
    </span>
  );
}

function Section({
  group,
  title,
  icon,
  rows,
  defaultOpen,
  ...props
}: RowListProps & {
  group: EmailGroup;
  title: string;
  icon: ReactNode;
  rows: JobRow[];
  defaultOpen: boolean;
}) {
  const [userOpen, setUserOpen] = useState<boolean | null>(null);
  const hasOpenRow = rows.some((row) => row.jobs[0].id === props.openRowId);
  const open = userOpen ?? (defaultOpen || hasOpenRow);

  return (
    <motion.section
      layout="position"
      transition={{ duration: 0.25, ease: EASE_OUT }}
      className={`${styles.section} ${styles[`tone_${group}`]}`}
    >
      <button type="button" className={styles.sectionHeader} aria-expanded={open} onClick={() => setUserOpen(!open)}>
        <span className={`${styles.sectionIcon} ${group === 'active' ? styles.spinChild : ''}`}>{icon}</span>
        <span className={styles.sectionTitle}>{title}</span>
        <span className={styles.sectionCount}>
          <RollingCount value={rows.length} />
        </span>
        <CaretRight size={16} className={`${styles.caret} ${open ? styles.caretOpen : ''}`} />
      </button>
      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            className={styles.sectionBody}
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.25, ease: EASE_OUT }}
          >
            <AnimatePresence initial={false}>
              {rows.map((row) => (
                <EmailRow
                  key={row.jobs[0].id}
                  row={row}
                  open={props.openRowId === row.jobs[0].id}
                  onToggle={() => props.onToggle(row.jobs[0].id)}
                />
              ))}
            </AnimatePresence>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.section>
  );
}

/** What this run did, so two syncs of the same mailbox read differently. */
function runTitle(run: JobRun, attention: number, created: number): string {
  if (run.kind === 'other') return jobLabel(run.root.type);
  if (run.kind === 'message') return run.root.label || 'Message';

  const emails = plural(run.emailCount, 'email', 'emails');
  const isImport = run.root.type === 'mailbox.backfill';
  if (run.reading) return isImport ? 'Starting your import' : 'Syncing your mailbox';
  if (run.active) return `${isImport ? 'Importing' : 'Processing'} ${emails}`;
  if (run.emailCount === 0) return attention > 0 ? "Couldn't read the mailbox" : 'No new emails';
  if (attention > 0) return `${attention} of ${emails} need${attention === 1 ? 's' : ''} attention`;
  if (isImport && created === 0) return `${emails} imported, no transactions`;
  if (created > 0) return `${plural(created, 'transaction', 'transactions')} from ${emails}`;
  return `${emails}, no transactions`;
}

function runMeta(run: JobRun): string {
  const time = formatTime(run.root.createdAt);
  if (run.kind === 'mailbox') return `${run.root.label || 'Mailbox'} · ${time}`;
  if (run.kind === 'message') return `Message · ${time}`;
  return `${STATE_LABELS[rowState({ jobs: [run.root], children: [] })]} · ${time}`;
}

/** The email being worked on now, with its steps filling in. */
function NowProcessing({ row }: { row: JobRow }) {
  return (
    <div className={styles.now}>
      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={row.jobs[0].id}
          className={styles.nowInner}
          initial={{ opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -10 }}
          transition={{ duration: 0.3, ease: EASE_OUT }}
        >
          <span className={styles.nowTitle}>{rowTitle(row)}</span>
          <StepLine steps={messageSteps(row)} />
        </motion.div>
      </AnimatePresence>
    </div>
  );
}

function Progress({ run, current }: { run: JobRun; current: JobRow | null }) {
  const total = run.emailCount;
  const percent = total > 0 ? Math.round((run.finished / total) * 100) : 0;
  return (
    <motion.div
      className={styles.progressWrap}
      initial={{ height: 0, opacity: 0 }}
      animate={{ height: 'auto', opacity: 1 }}
      exit={{ height: 0, opacity: 0 }}
      transition={{ duration: 0.3, ease: EASE_OUT }}
    >
      <div className={styles.progress}>
        <span className={styles.progressText}>
          {run.reading ? (
            <span>Reading the mailbox</span>
          ) : (
            <span>
              <RollingCount value={run.finished} /> of {total} emails
            </span>
          )}
          {!run.reading && <span className={styles.muted}>{run.doing ? `${run.doing}…` : 'Waiting…'}</span>}
        </span>
        <div
          className={`${styles.bar} ${run.reading ? styles.barReading : ''}`}
          role="progressbar"
          aria-label="Emails processed"
          aria-valuenow={run.reading ? undefined : run.finished}
          aria-valuemin={0}
          aria-valuemax={total}
        >
          <motion.span
            initial={false}
            animate={run.reading ? undefined : { width: `${percent}%` }}
            transition={{ duration: 0.6, ease: EASE_OUT }}
          />
        </div>
        {current?.jobs[0].type === 'message.received' && <NowProcessing row={current} />}
      </div>
    </motion.div>
  );
}

function RunView({
  run,
  show,
  query,
  forceOpen,
  ...props
}: RowListProps & {
  run: JobRun;
  show: ShowFilter;
  query: string;
  forceOpen: boolean;
}) {
  const [userOpen, setUserOpen] = useState<boolean | null>(null);

  const groups = useMemo(() => {
    const byGroup: Record<EmailGroup, JobRow[]> = {
      attention: [],
      active: [],
      created: [],
      skipped: [],
      cancelled: [],
    };
    for (const row of run.rows) byGroup[emailGroup(row)].push(row);
    return byGroup;
  }, [run.rows]);

  const needle = query.trim().toLowerCase();
  const visible = (group: EmailGroup): JobRow[] => {
    if (show !== 'all' && show !== group) return [];
    if (!needle) return groups[group];
    return groups[group].filter((row) => rowTitle(row).toLowerCase().includes(needle));
  };
  const shownRows = (Object.keys(groups) as EmailGroup[]).flatMap(visible);
  const filtering = show !== 'all' || needle !== '';
  if (filtering && shownRows.length === 0) return null;

  const hasOpenRow = shownRows.some((row) => row.jobs[0].id === props.openRowId);
  const open = userOpen ?? (run.active || groups.attention.length > 0 || forceOpen || filtering || hasOpenRow);
  const current =
    groups.active.find((row) => rowJobs(row).some((job) => job.status === 'running')) ?? groups.active[0] ?? null;

  const shownSections = SECTIONS.filter(({ group }) => visible(group).length > 0);
  const isMain = ({ openByDefault }: (typeof SECTIONS)[number]) => openByDefault;
  const main = shownSections.filter(isMain);
  const side = shownSections.filter((section) => !isMain(section));
  const split = main.length > 0 && side.length > 0;
  const renderSection = ({ group, title, icon, openByDefault }: (typeof SECTIONS)[number]) => (
    <Section
      key={group}
      group={group}
      title={title}
      icon={icon}
      rows={visible(group)}
      defaultOpen={openByDefault || filtering}
      {...props}
    />
  );

  return (
    <section className={`${styles.run} ${run.active ? styles.runActive : ''}`}>
      <button type="button" className={styles.runHeader} aria-expanded={open} onClick={() => setUserOpen(!open)}>
        <span className={styles.runIcon}>
          {run.active ? (
            <CircleNotch size={20} className={styles.spin} />
          ) : groups.attention.length > 0 ? (
            <Warning size={20} className={styles.warn} />
          ) : run.kind === 'other' ? (
            <CheckCircle size={20} className={styles.muted} />
          ) : (
            <EnvelopeSimple size={20} className={styles.muted} />
          )}
        </span>
        <span className={styles.runText}>
          <span className={styles.runTitle}>{runTitle(run, groups.attention.length, groups.created.length)}</span>
          <span className={styles.runMeta}>{runMeta(run)}</span>
        </span>
        <CaretRight size={16} className={`${styles.caret} ${open ? styles.caretOpen : ''}`} />
      </button>

      <AnimatePresence initial={false}>
        {run.active && run.kind === 'mailbox' && <Progress key="progress" run={run} current={current} />}
      </AnimatePresence>

      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            className={styles.runBodyWrap}
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.25, ease: EASE_OUT }}
          >
            <div className={`${styles.runBody} ${split ? styles.runBodySplit : ''}`}>
              {main.length > 0 && <div className={styles.runColumn}>{main.map(renderSection)}</div>}
              {side.length > 0 && <div className={styles.runColumn}>{side.map(renderSection)}</div>}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </section>
  );
}

export default function Jobs() {
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const [retryingCode, setRetryingCode] = useState<string | null>(null);
  const dispatch = useAppDispatch();
  const { setHeaderContent } = useHeader();
  const { list, listLoading, error, filters, updatedAt } = useAppSelector((state) => state.jobs);
  const [openRowId, setOpenRowId] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [show, setShow] = useState<ShowFilter>('all');
  const [refreshes, setRefreshes] = useState(0);
  const [searchParams, setSearchParams] = useSearchParams();
  const view = searchParams.get('view') === 'pipeline' ? 'pipeline' : 'emails';
  const changeView = (next: 'emails' | 'pipeline') =>
    setSearchParams(
      (params) => {
        if (next === 'pipeline') params.set('view', 'pipeline');
        else params.delete('view');
        return params;
      },
      { replace: true },
    );

  useEffect(() => {
    setHeaderContent(null);
  }, [setHeaderContent]);

  useEffect(() => {
    dispatch(fetchJobs());
  }, [dispatch, filters]);

  useEffect(
    () => () => {
      dispatch(jobDetailsCleared());
      // The filters live in the store, so they'd still hide jobs on the next visit.
      dispatch(jobFiltersChanged({ errorCode: '', jobId: '' }));
    },
    [dispatch],
  );

  // A failure that's been fixed loses its chip, and with it the only way to
  // turn its filter off.
  useEffect(() => {
    if (list && filters.errorCode && !list.failures.some((failure) => failure.code === filters.errorCode)) {
      dispatch(jobFiltersChanged({ errorCode: '' }));
    }
  }, [dispatch, list, filters.errorCode]);

  const counts = list?.counts;
  const isActive = !!counts && counts.running + counts.queued + counts.retrying > 0;
  const refreshing = listLoading === LoadingState.PENDING;

  const allRuns = useMemo(() => (list?.trees ?? []).flatMap((tree) => buildRun(tree) ?? []), [list?.trees]);
  const isEmptySync = (run: JobRun) => run.kind === 'mailbox' && run.rows.length === 0 && !run.active;
  const emptySyncs = allRuns.filter(isEmptySync).length;
  const runs = allRuns.filter((run) => !isEmptySync(run));

  const changeSearch = (value: string) => {
    setSearch(value);
    const jobId = UUID_PATTERN.test(value.trim()) ? value.trim() : '';
    if (jobId !== filters.jobId) dispatch(jobFiltersChanged({ jobId }));
  };
  const toggleRow = (id: string) => setOpenRowId((current) => (current === id ? null : id));

  const retryAll = async (code: string) => {
    setRetryingCode(code);
    try {
      const retried = await dispatch(retryFailedJobs(code)).unwrap();
      toast.success(retried > 0 ? `${plural(retried, 'job', 'jobs')} queued again` : 'Nothing has failed for good yet');
    } catch (err) {
      toast.error("Couldn't retry those jobs", { description: errorMessage(err, 'Please try again.') });
    } finally {
      setRetryingCode(null);
    }
  };

  const total = list?.total ?? 0;
  const failures = list?.failures ?? [];
  const query = filters.jobId ? '' : search;
  const anyAttention = runs.some((run) => run.rows.some((row) => emailGroup(row) === 'attention'));
  const filtering = show !== 'all' || search.trim() !== '' || filters.errorCode !== '';

  return (
    <MotionConfig reducedMotion="user">
      <div className={styles.page}>
        <div className={styles.views} role="tablist" aria-label="Jobs view">
          {(
            [
              ['emails', 'Emails'],
              ['pipeline', 'Pipelines'],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={view === id}
              className={`${styles.view} ${view === id ? styles.viewOn : ''}`}
              onClick={() => changeView(id)}
            >
              {label}
            </button>
          ))}
        </div>

        {view === 'pipeline' ? (
          <PipelineView />
        ) : list && !list.enabled ? (
          <div className={styles.empty}>
            The job queue is off. Set <code>JOB_DRIVER=postgres</code> on the API to see jobs here.
          </div>
        ) : (
          <>
            <div className={styles.toolbar}>
              <label className={styles.search}>
                <MagnifyingGlass size={16} />
                <input
                  aria-label="Search by subject or job ID"
                  placeholder="Search subject or paste a job ID"
                  value={search}
                  onChange={(event) => changeSearch(event.target.value)}
                />
              </label>
              <select
                className={styles.select}
                aria-label="Show"
                value={show}
                onChange={(event) => setShow(event.target.value as ShowFilter)}
              >
                {SHOW_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
              <span className={styles.updated}>
                {isActive && <span className={styles.liveDotOn} />}
                {updatedAt && `Updated ${formatClock(updatedAt)}`}
              </span>
              <button
                type="button"
                className={styles.button}
                disabled={refreshing}
                onClick={() => {
                  setRefreshes((count) => count + 1);
                  dispatch(fetchJobs());
                }}
              >
                <motion.span
                  className={styles.refreshIcon}
                  animate={{ rotate: refreshes * 360 }}
                  transition={{ duration: 0.6, ease: EASE_OUT }}
                >
                  <ArrowsClockwise size={16} />
                </motion.span>
                Refresh
              </button>
            </div>

            {failures.length > 0 && (
              <div className={styles.failures}>
                <h4 className={styles.groupTitle}>Failing now</h4>
                {failures.map((failure) => (
                  <div key={failure.code} className={styles.failureLine}>
                    <button
                      type="button"
                      className={`${styles.failure} ${filters.errorCode === failure.code ? styles.failureOn : ''}`}
                      aria-pressed={filters.errorCode === failure.code}
                      onClick={() =>
                        dispatch(
                          jobFiltersChanged({
                            errorCode: filters.errorCode === failure.code ? '' : failure.code,
                          }),
                        )
                      }
                    >
                      <span className={styles.failureText} title={failure.message}>
                        {failure.code}
                      </span>
                      <span className={styles.failureMeta}>
                        <span className={styles.muted}>
                          {plural(failure.count, 'job', 'jobs')} · {formatAgo(failure.lastSeen)}
                        </span>
                        {filters.errorCode === failure.code && <X size={14} weight="bold" aria-hidden="true" />}
                      </span>
                    </button>
                    {!isDemoUser && (
                      <button
                        type="button"
                        className={styles.button}
                        disabled={retryingCode !== null}
                        onClick={() => void retryAll(failure.code)}
                      >
                        <ArrowsClockwise
                          size={16}
                          className={retryingCode === failure.code ? styles.spin : undefined}
                        />
                        Retry all
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )}

            {filters.errorCode && (
              <p className={styles.filterNote}>
                <span>
                  Only showing jobs failing with <code>{filters.errorCode}</code>
                </span>
                <button
                  type="button"
                  className={styles.textToggle}
                  onClick={() => dispatch(jobFiltersChanged({ errorCode: '' }))}
                >
                  Show all jobs
                </button>
              </p>
            )}

            {list && runs.length > 0 && !anyAttention && !isActive && !filtering && (
              <p className={styles.allClear}>
                <CheckCircle size={18} />
                Nothing needs attention.
              </p>
            )}

            {listLoading === LoadingState.ERROR && <div className={styles.empty}>{error}</div>}
            {!list && listLoading === LoadingState.PENDING && <div className={styles.empty}>Loading jobs…</div>}
            {list && runs.length === 0 && listLoading !== LoadingState.ERROR && (
              <div className={styles.empty}>{filtering ? 'Nothing matches.' : 'No jobs yet.'}</div>
            )}

            <div className={styles.runs}>
              {runs.map((run) => (
                <RunView
                  key={run.id}
                  run={run}
                  show={show}
                  query={query}
                  forceOpen={filters.jobId !== '' || filters.errorCode !== ''}
                  openRowId={openRowId}
                  onToggle={toggleRow}
                />
              ))}
            </div>

            {emptySyncs > 0 && !filtering && (
              <p className={styles.footnote}>{plural(emptySyncs, 'sync', 'syncs')} on this page found no new email.</p>
            )}

            {total > JOBS_PAGE_SIZE && (
              <div className={styles.pager}>
                <button
                  type="button"
                  className={styles.button}
                  disabled={filters.offset === 0}
                  onClick={() => dispatch(jobFiltersChanged({ offset: Math.max(0, filters.offset - JOBS_PAGE_SIZE) }))}
                >
                  Newer
                </button>
                <span className={styles.muted}>
                  {filters.offset + 1}–{Math.min(filters.offset + JOBS_PAGE_SIZE, total)} of {total}
                </span>
                <button
                  type="button"
                  className={styles.button}
                  disabled={filters.offset + JOBS_PAGE_SIZE >= total}
                  onClick={() => dispatch(jobFiltersChanged({ offset: filters.offset + JOBS_PAGE_SIZE }))}
                >
                  Older
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </MotionConfig>
  );
}
