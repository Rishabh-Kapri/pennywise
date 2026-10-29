import { useEffect, useRef, useState } from 'react';
import { ArrowLeft, ArrowRight, CheckCircle, Envelope, ShieldCheck } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { apiClient, errorMessage } from '@/utils';
import type { PipelineRun } from '@/features/pipeline/types/pipeline.types';
import { fetchAIProviders, startEmailBackfill } from '../../api';
import type { StepProps } from '../OnboardingWizard';
import styles from './steps.module.css';

const BACKFILL_DAYS = 14;
const POLL_MS = 3000;
// waiting_retry: some emails are parked for a retry (Settings → Activity).
const TERMINAL = new Set(['completed', 'failed', 'waiting_retry']);

type Phase = 'idle' | 'starting' | 'running' | 'finished' | 'empty' | 'error';

/**
 * Shows the Gmail connection and runs the pipeline over the last two weeks of
 * bank alerts, so the user sees automation work (and catches a wrong account
 * suffix) before they rely on it.
 */
export function EmailStep({ onNext, onBack, goTo, busy }: StepProps) {
  const email = useAppSelector((state) => state.auth.user?.email);
  const [hasProvider, setHasProvider] = useState<boolean | null>(null);
  const [phase, setPhase] = useState<Phase>('idle');
  const [emailCount, setEmailCount] = useState(0);
  const [run, setRun] = useState<PipelineRun | null>(null);
  const [error, setError] = useState<string | null>(null);
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => {
    fetchAIProviders()
      .then((list) => setHasProvider((list ?? []).some((provider) => provider.enabled)))
      .catch(() => setHasProvider(false));
    return () => window.clearTimeout(timer.current);
  }, []);

  const poll = (workflowId: string) => {
    timer.current = window.setTimeout(async () => {
      try {
        const runs = await apiClient.get<PipelineRun[]>('pipeline/runs?limit=20');
        const found = runs?.find((r) => r.workflowId === workflowId);
        if (found) setRun(found);
        if (found && TERMINAL.has(found.status)) {
          setPhase('finished');
          return;
        }
      } catch {
        // A missed poll is fine; try again.
      }
      poll(workflowId);
    }, POLL_MS);
  };

  const start = async () => {
    setPhase('starting');
    setError(null);
    try {
      const res = await startEmailBackfill(BACKFILL_DAYS);
      setEmailCount(res.emailCount);
      if (res.emailCount === 0 || !res.workflowId) {
        setPhase('empty');
        return;
      }
      setPhase('running');
      poll(res.workflowId);
    } catch (err) {
      setError(errorMessage(err, 'Could not import your emails.'));
      setPhase('error');
    }
  };

  const processed = run ? run.transactionsCreated + run.emailsSkipped : 0;
  const percent = emailCount ? Math.min(100, Math.round((processed / emailCount) * 100)) : 0;

  return (
    <>
      <p className={styles.eyebrow}>Step 4 · Optional</p>
      <h1 className={styles.title}>Turn bank emails into transactions</h1>
      <p className={styles.lead}>
        New alerts are picked up automatically from now on. Import the last {BACKFILL_DAYS} days to see it work and to
        check your accounts are matched correctly.
      </p>

      <div className={`${styles.section} ${styles.panel}`}>
        <div className={styles.listItem} style={{ border: 0, padding: 0 }}>
          <Envelope size={22} weight="duotone" />
          <div className={styles.grow}>
            <strong>Gmail connected</strong>
            <small>{email ?? 'Your Google account'}</small>
          </div>
          <CheckCircle size={20} weight="fill" color="var(--color-success)" />
        </div>
        <div className={styles.callout}>
          <ShieldCheck size={18} weight="bold" aria-hidden="true" />
          <span>
            Only emails that look like bank alerts (debited, credited, card transaction…) are read, and only their text
            goes to your AI provider. Nothing else in your inbox is touched.
          </span>
        </div>
      </div>

      {hasProvider === false && (
        <div className={`${styles.callout} ${styles.calloutWarning}`}>
          <span>
            Importing needs an AI provider.{' '}
            <button type="button" className={styles.ghost} style={{ padding: 0 }} onClick={() => goTo('ai')}>
              Add one now
            </button>
          </span>
        </div>
      )}

      {hasProvider && phase === 'idle' && (
        <div className={styles.actions} style={{ marginTop: '1rem' }}>
          <button type="button" className={styles.secondary} onClick={start}>
            Import the last {BACKFILL_DAYS} days
          </button>
        </div>
      )}

      {phase === 'starting' && <div className={styles.callout}>Looking for bank emails…</div>}

      {phase === 'empty' && (
        <div className={styles.callout}>
          No bank alert emails in the last {BACKFILL_DAYS} days. New ones will still be picked up as they arrive.
        </div>
      )}

      {(phase === 'running' || phase === 'finished') && (
        <div className={`${styles.section} ${styles.panel}`}>
          <strong>
            {phase === 'finished' ? 'Done' : 'Reading'} {emailCount} bank email{emailCount === 1 ? '' : 's'}
          </strong>
          <div className={styles.progress}>
            <div className={styles.progressBar} style={{ width: `${phase === 'finished' ? 100 : percent}%` }} />
          </div>
          <p className={styles.hint} style={{ marginTop: '0.75rem' }}>
            {run?.transactionsCreated ?? 0} transaction{run?.transactionsCreated === 1 ? '' : 's'} added
            {run?.emailsSkipped ? ` · ${run.emailsSkipped} skipped (not a transaction, or no matching account)` : ''}
          </p>
          {phase === 'finished' && run?.status === 'waiting_retry' && (
            <div className={`${styles.callout} ${styles.calloutWarning}`}>
              Some emails couldn't be processed. They're waiting in Settings → Activity, where you can retry them.
            </div>
          )}
          {phase === 'finished' && run?.status === 'failed' && (
            <div className={`${styles.callout} ${styles.calloutError}`}>{run.error ?? 'The import failed.'}</div>
          )}
          {phase === 'finished' && (
            <p className={styles.hint} style={{ marginTop: '0.5rem' }}>
              Imported transactions wait in Transactions for your approval. Fixing a payee or category there teaches
              Pennywise for next time. Per-email details are in Settings → Activity.
            </p>
          )}
        </div>
      )}

      {phase === 'error' && error && <div className={`${styles.callout} ${styles.calloutError}`}>{error}</div>}

      <div className={styles.actions}>
        <button type="button" className={styles.ghost} onClick={onBack} disabled={busy}>
          <ArrowLeft size={16} /> Back
        </button>
        <span className={styles.spacer} />
        {phase !== 'finished' && (
          <button type="button" className={styles.ghost} onClick={() => onNext({ skipped: phase === 'idle' || phase === 'error' })} disabled={busy}>
            {phase === 'running' ? 'Continue in the background' : 'Skip'}
          </button>
        )}
        {phase === 'finished' || phase === 'empty' ? (
          <button type="button" className={styles.primary} onClick={() => onNext()} disabled={busy}>
            Continue <ArrowRight size={16} />
          </button>
        ) : null}
      </div>
    </>
  );
}
