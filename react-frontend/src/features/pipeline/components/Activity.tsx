import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useHeader } from '@/context/HeaderContext';
import { selectIsDemoUser } from '@/features/auth/store/authSlice';
import { LoadingState, toast } from '@/utils';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import {
  ArrowsClockwise,
  CaretDown,
  CaretRight,
  CheckCircle,
  CircleNotch,
  EnvelopeSimple,
  Warning,
  XCircle,
} from '@phosphor-icons/react';
import { useEffect, useMemo, useState } from 'react';
import {
  fetchPipelineRunDetail,
  fetchPipelineRuns,
  retryPipelineRun,
} from '../store';
import type {
  PipelineRun,
  PipelineRunEvent,
  PipelineStep,
} from '../types/pipeline.types';
import styles from './Activity.module.css';

const STEPPER: { key: PipelineStep; label: string }[] = [
  { key: 'fetch_emails', label: 'Fetch' },
  { key: 'parse', label: 'Extract' },
  { key: 'predict', label: 'Predict' },
  { key: 'create_transactions', label: 'Create' },
];

const STEP_ORDER: PipelineStep[] = [
  'fetch_user',
  'fetch_emails',
  'parse',
  'predict',
  'create_transactions',
  'done',
];

type StepState = 'done' | 'active' | 'waiting' | 'error' | 'pending';

function stepState(run: PipelineRun, step: PipelineStep): StepState {
  if (run.status === 'completed') {
    return 'done';
  }
  // fetch_user is preparation for the fetch step, show it under "Fetch"
  const currentStep = run.currentStep === 'fetch_user' ? 'fetch_emails' : run.currentStep;
  const currentIndex = STEP_ORDER.indexOf(currentStep);
  const stepIndex = STEP_ORDER.indexOf(step);

  if (stepIndex < currentIndex) {
    return 'done';
  }
  if (stepIndex > currentIndex) {
    return 'pending';
  }
  if (run.status === 'failed') {
    return 'error';
  }
  if (run.status === 'waiting_retry') {
    return 'waiting';
  }
  return 'active';
}

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const diffMs = Date.now() - date.getTime();
  const diffMinutes = Math.floor(diffMs / 60_000);

  if (diffMinutes < 1) return 'just now';
  if (diffMinutes < 60) return `${diffMinutes}m ago`;
  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours}h ago`;

  return date.toLocaleString(undefined, {
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  });
}

const STATUS_LABELS: Record<PipelineRun['status'], string> = {
  running: 'Running',
  waiting_retry: 'Needs retry',
  completed: 'Completed',
  failed: 'Failed',
};

function detailString(event: PipelineRunEvent | undefined, key: string): string {
  const value = event?.detail?.[key];
  if (value === undefined || value === null) return '';
  return String(value);
}

function detailNumber(event: PipelineRunEvent | undefined, key: string): number | null {
  const value = event?.detail?.[key];
  return typeof value === 'number' ? value : null;
}

interface EmailGroup {
  messageId: string;
  fetch?: PipelineRunEvent;
  parse?: PipelineRunEvent;
  predict?: PipelineRunEvent;
}

function groupEventsByEmail(events: PipelineRunEvent[]): EmailGroup[] {
  const groups = new Map<string, EmailGroup>();
  for (const event of events) {
    if (!event.messageId) continue;
    let group = groups.get(event.messageId);
    if (!group) {
      group = { messageId: event.messageId };
      groups.set(event.messageId, group);
    }
    if (event.step === 'fetch_emails') group.fetch = event;
    if (event.step === 'parse') group.parse = event;
    if (event.step === 'predict') group.predict = event;
  }
  return [...groups.values()];
}

/** "Bank Alerts <alerts@bank.com>" → "Bank Alerts"; bare addresses pass through. */
function formatSender(from: string): string {
  const match = from.match(/^\s*"?([^"<]*?)"?\s*<(.+)>\s*$/);
  if (!match) return from.trim();
  return match[1].trim() || match[2].trim();
}

function senderAddress(from: string): string {
  const match = from.match(/<(.+)>/);
  return match ? match[1].trim() : '';
}

function StatusChip({ status }: { status: PipelineRun['status'] }) {
  return (
    <span className={`${styles.statusChip} ${styles[`status_${status}`]}`}>
      {status === 'running' && <CircleNotch className={styles.spin} size={14} />}
      {status === 'waiting_retry' && <Warning size={14} />}
      {status === 'completed' && <CheckCircle size={14} />}
      {status === 'failed' && <XCircle size={14} />}
      {STATUS_LABELS[status]}
    </span>
  );
}

function Stepper({ run }: { run: PipelineRun }) {
  return (
    <div className={styles.stepper}>
      {STEPPER.map((step, index) => {
        const state = stepState(run, step.key);
        return (
          <div key={step.key} className={styles.stepperItem}>
            {index > 0 && <span className={`${styles.stepConnector} ${styles[`conn_${state}`]}`} />}
            <span className={`${styles.stepDot} ${styles[`step_${state}`]}`} />
            <span className={`${styles.stepLabel} ${state === 'pending' ? styles.stepLabelPending : ''}`}>
              {step.label}
            </span>
          </div>
        );
      })}
    </div>
  );
}

function EmailCard({ group }: { group: EmailGroup }) {
  const [showReasoning, setShowReasoning] = useState(false);

  const parseSkipped = group.parse?.status === 'skipped';
  const parseFailed = group.parse?.status === 'failed';
  const predictFailed = group.predict?.status === 'failed';
  const amount = detailNumber(group.parse, 'amount');
  const source = detailString(group.predict, 'source');
  const reasoning = detailString(group.predict, 'reasoning');

  // Captured at fetch time, so it is available before extraction runs.
  const from = detailString(group.fetch, 'from');
  const subject = detailString(group.fetch, 'subject');
  const snippet = detailString(group.fetch, 'snippet');
  const address = senderAddress(from);

  return (
    <div className={styles.emailCard}>
      {group.fetch && (
        <div className={styles.emailSource}>
          <div className={styles.emailSourceTop}>
            <EnvelopeSimple size={16} className={styles.emailIcon} />
            <span className={styles.emailSender} title={from}>
              {from ? formatSender(from) : 'Unknown sender'}
            </span>
            {address && <span className={styles.emailAddress}>{address}</span>}
            <span className={styles.messageId} title={`Message ID: ${group.messageId}`}>
              {group.messageId}
            </span>
          </div>
          {subject && <div className={styles.emailSubject}>{subject}</div>}
          {snippet && <p className={styles.emailSnippet}>{snippet}</p>}
        </div>
      )}
      {!group.parse && !group.predict && (
        <div className={styles.emailPending}>
          <CircleNotch size={14} className={styles.spin} />
          Waiting for extraction…
        </div>
      )}
      {(group.parse || group.predict) && (
        <div className={styles.emailCardHeader}>
          {!group.fetch && <EnvelopeSimple size={16} className={styles.emailIcon} />}
          {parseSkipped ? (
            <span className={styles.mutedText}>Skipped — not a transaction email</span>
          ) : parseFailed ? (
            <span className={styles.errorText}>
              <XCircle size={14} /> Extraction failed for this email
            </span>
          ) : (
            <>
              <span className={styles.emailMerchant}>
                {detailString(group.parse, 'merchant') || 'Unknown merchant'}
              </span>
              {amount !== null && (
                <span className={styles.emailAmount}>{getCurrencyLocaleString(amount)}</span>
              )}
            </>
          )}
        </div>
      )}
      {!parseSkipped && !parseFailed && group.parse && (
        <div className={styles.emailMetaRow}>
          <span>{detailString(group.parse, 'account')}</span>
          <span>{detailString(group.parse, 'date')}</span>
          <span>{detailString(group.parse, 'transactionType')}</span>
        </div>
      )}
      {group.predict && !parseSkipped && (
        <div className={styles.predictionRow}>
          {predictFailed ? (
            <span className={styles.errorText}>
              <XCircle size={14} /> Prediction failed for this email
            </span>
          ) : (
            <>
              <span>
                {detailString(group.predict, 'payee') || 'Unknown payee'}
                <span className={styles.mutedText}> → </span>
                {detailString(group.predict, 'category') || 'Uncategorized'}
              </span>
              {source && <span className={styles.sourceBadge}>{source.toLowerCase()}</span>}
              {detailString(group.predict, 'confidence') && (
                <span className={styles.mutedText}>
                  {detailString(group.predict, 'confidence')} confidence
                </span>
              )}
            </>
          )}
        </div>
      )}
      {reasoning && !predictFailed && !parseSkipped && (
        <div className={styles.reasoning}>
          <button
            type="button"
            className={styles.reasoningToggle}
            onClick={() => setShowReasoning((prev) => !prev)}>
            {showReasoning ? <CaretDown size={12} /> : <CaretRight size={12} />}
            reasoning
          </button>
          {showReasoning && <p>{reasoning}</p>}
        </div>
      )}
    </div>
  );
}

function RunTimeline({ events }: { events: PipelineRunEvent[] }) {
  const runLevelEvents = events.filter((event) => !event.messageId);
  if (runLevelEvents.length === 0) return null;

  return (
    <div className={styles.timeline}>
      {runLevelEvents.map((event) => (
        <div key={event.id} className={styles.timelineRow}>
          <span className={styles.timelineTime}>
            {new Date(event.createdAt).toLocaleTimeString(undefined, {
              hour: '2-digit',
              minute: '2-digit',
              second: '2-digit',
            })}
          </span>
          <span className={styles.timelineStep}>{event.step.replace(/_/g, ' ')}</span>
          <span className={`${styles.timelineStatus} ${styles[`event_${event.status}`]}`}>
            {event.status.replace(/_/g, ' ')}
          </span>
          {detailString(event, 'error') && (
            <span className={styles.timelineError}>{detailString(event, 'error')}</span>
          )}
        </div>
      ))}
    </div>
  );
}

function RunCard({
  run,
  isExpanded,
  onToggle,
}: {
  run: PipelineRun;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const dispatch = useAppDispatch();
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const events = useAppSelector((state) => state.pipeline.eventsByRunId[run.id]);
  const detailLoading = useAppSelector((state) => state.pipeline.detailLoading);
  const retryingRunId = useAppSelector((state) => state.pipeline.retryingRunId);

  useEffect(() => {
    if (isExpanded) {
      dispatch(fetchPipelineRunDetail(run.id));
    }
    // refetch the timeline when the run row changes while expanded
  }, [dispatch, isExpanded, run.id, run.updatedAt]);

  const handleRetry = async () => {
    try {
      await dispatch(retryPipelineRun(run.id)).unwrap();
      toast.success('Retry signal sent');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to retry run');
    }
  };

  const emailGroups = useMemo(() => groupEventsByEmail(events ?? []), [events]);

  const counts: string[] = [];
  if (run.emailsFetched > 0) {
    counts.push(`${run.emailsFetched} email${run.emailsFetched === 1 ? '' : 's'}`);
  }
  if (run.emailsSkipped > 0) {
    counts.push(`${run.emailsSkipped} skipped`);
  }
  if (run.transactionsCreated > 0) {
    counts.push(
      `${run.transactionsCreated} transaction${run.transactionsCreated === 1 ? '' : 's'}`,
    );
  }
  if (run.status === 'completed' && run.emailsFetched === 0) {
    counts.push('no transaction emails');
  }

  return (
    <div className={styles.runCard}>
      <button type="button" className={styles.runCardHeader} onClick={onToggle}>
        <span className={styles.runCaret}>
          {isExpanded ? <CaretDown size={14} /> : <CaretRight size={14} />}
        </span>
        <StatusChip status={run.status} />
        <span className={styles.runAccount}>
          {run.emailAccount ?? (run.trigger === 'manual' ? 'manual run' : 'email')}
        </span>
        <span className={styles.runCounts}>{counts.join(' · ')}</span>
        {run.gmailHistoryId !== undefined && (
          <span className={styles.historyChip} title="Gmail history ID">
            history {run.gmailHistoryId}
          </span>
        )}
        <span className={styles.runTime}>{formatRelativeTime(run.startedAt)}</span>
      </button>

      <div className={styles.runCardBody}>
        <Stepper run={run} />
        {run.error && run.status !== 'completed' && (
          <div className={styles.errorBox}>
            <span className={styles.errorText}>
              <Warning size={14} /> {run.error}
            </span>
            {run.status === 'waiting_retry' && !isDemoUser && (
              <button
                type="button"
                className={styles.retryButton}
                disabled={retryingRunId === run.id}
                onClick={handleRetry}>
                <ArrowsClockwise
                  size={14}
                  className={retryingRunId === run.id ? styles.spin : undefined}
                />
                Retry
              </button>
            )}
          </div>
        )}
      </div>

      {isExpanded && (
        <div className={styles.runDetail}>
          {events === undefined && detailLoading === LoadingState.PENDING ? (
            <span className={styles.mutedText}>Loading timeline…</span>
          ) : (
            <>
              {emailGroups.length > 0 && (
                <div className={styles.emailCards}>
                  {emailGroups.map((group) => (
                    <EmailCard key={group.messageId} group={group} />
                  ))}
                </div>
              )}
              <RunTimeline events={events ?? []} />
              {(events ?? []).length === 0 && (
                <span className={styles.mutedText}>No timeline recorded for this run.</span>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}

export default function Activity() {
  const dispatch = useAppDispatch();
  const { setHeaderContent } = useHeader();
  const { runs, runsLoading, error } = useAppSelector((state) => state.pipeline);
  const [expandedRunId, setExpandedRunId] = useState<string | null>(null);

  useEffect(() => {
    setHeaderContent(null);
    dispatch(fetchPipelineRuns());
  }, [dispatch, setHeaderContent]);

  const activeCount = runs.filter(
    (run) => run.status === 'running' || run.status === 'waiting_retry',
  ).length;

  return (
    <div className={styles.page}>
      <div className={styles.heading}>
        <div>
          <div className={styles.kicker}>email pipeline</div>
          <h1>Activity</h1>
          <p>
            Transaction emails as they move through fetch, extraction, prediction
            and transaction creation.
          </p>
        </div>
        {activeCount > 0 && (
          <span className={styles.activeBadge}>
            <CircleNotch className={styles.spin} size={14} />
            {activeCount} active
          </span>
        )}
      </div>

      {runsLoading === LoadingState.PENDING && runs.length === 0 && (
        <div className={styles.emptyState}>Loading runs…</div>
      )}
      {runsLoading === LoadingState.ERROR && (
        <div className={styles.emptyState}>{error}</div>
      )}
      {runsLoading === LoadingState.SUCCESS && runs.length === 0 && (
        <div className={styles.emptyState}>
          No pipeline runs yet. When a transaction email arrives, its progress
          will show up here.
        </div>
      )}

      <div className={styles.runList}>
        {runs.map((run) => (
          <RunCard
            key={run.id}
            run={run}
            isExpanded={expandedRunId === run.id}
            onToggle={() =>
              setExpandedRunId((prev) => (prev === run.id ? null : run.id))
            }
          />
        ))}
      </div>
    </div>
  );
}
