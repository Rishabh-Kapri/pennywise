import { useEffect, useState } from 'react';
import { ArrowLeft, ArrowRight, CheckCircle, CircleNotch, Envelope, ShieldCheck, Warning } from '@phosphor-icons/react';
import { EmailBackfill } from '@/features/mail/EmailBackfill';
import { useMailConnections } from '@/features/mail/useMailConnections';
import { useOnboarding } from '../../hooks/useOnboarding';
import { fetchHasChatModel } from '../../api';
import type { StepProps } from '../OnboardingWizard';
import styles from './steps.module.css';

/** Connects the Gmail account that receives the user's bank alerts. */
export function EmailStep({ onNext, onBack, goTo, busy }: StepProps) {
  const { connections, loadError, connecting, connect } = useMailConnections();
  const { onboarding, update } = useOnboarding();
  const [hasProvider, setHasProvider] = useState<boolean | null>(null);
  const connected = connections?.find((connection) => connection.status === 'active');
  const broken = connections?.find((connection) => connection.status !== 'active');

  useEffect(() => {
    fetchHasChatModel()
      .then(setHasProvider)
      .catch(() => setHasProvider(false));
  }, []);

  return (
    <>
      <p className={styles.eyebrow}>Step 4 · Optional</p>
      <h1 className={styles.title}>Turn bank emails into transactions</h1>
      <p className={styles.lead}>
        Connect the Gmail account that receives your bank alerts, and new ones become transactions for you to approve.
        You can also import the last two weeks.
      </p>

      <div className={`${styles.section} ${styles.panel}`}>
        <div className={styles.listItem} style={{ border: 0, padding: 0 }}>
          <Envelope size={22} weight="duotone" />
          <div className={styles.grow}>
            <strong>{connected ? 'Gmail connected' : broken ? 'Gmail needs reconnecting' : 'Connect Gmail'}</strong>
            <small>{connected?.account ?? broken?.account ?? 'Choose the account that gets your bank alerts'}</small>
          </div>
          {connected && <CheckCircle size={20} weight="fill" color="var(--color-success)" />}
          {!connected && broken && <Warning size={20} weight="fill" color="var(--color-warning)" />}
          {!connected && connections && (
            <button
              type="button"
              className={styles.secondary}
              disabled={connecting}
              onClick={() => connect(broken?.account)}
            >
              {connecting && <CircleNotch size={16} className={styles.spin} />}
              {broken ? 'Reconnect' : 'Connect'}
            </button>
          )}
        </div>
        {connected && hasProvider && onboarding?.emailsImportedAt && (
          <p className={styles.hint} style={{ marginTop: '1rem' }}>
            Your import has started. Follow it in Settings → Jobs.
          </p>
        )}
        {connected && hasProvider && !onboarding?.emailsImportedAt && (
          <EmailBackfill connectionId={connected.id} onImported={() => update({ emailsImported: true })} />
        )}
        <div className={styles.callout}>
          <ShieldCheck size={18} weight="bold" aria-hidden="true" />
          <span>
            Pennywise gets read-only access, so it can't send, change or delete your mail. New emails are checked to
            find bank alerts.
          </span>
        </div>
      </div>

      {loadError && <div className={`${styles.callout} ${styles.calloutError}`}>{loadError}</div>}

      {connected && hasProvider === false && (
        <div className={`${styles.callout} ${styles.calloutWarning}`}>
          <span>
            Reading emails needs an AI provider.{' '}
            <button type="button" className={styles.ghost} style={{ padding: 0 }} onClick={() => goTo('ai')}>
              Add one now
            </button>
          </span>
        </div>
      )}

      <div className={styles.actions}>
        <button type="button" className={styles.ghost} onClick={onBack} disabled={busy}>
          <ArrowLeft size={16} /> Back
        </button>
        <span className={styles.spacer} />
        {connected ? (
          <button type="button" className={styles.primary} onClick={() => onNext()} disabled={busy}>
            Continue <ArrowRight size={16} />
          </button>
        ) : (
          <button type="button" className={styles.ghost} onClick={() => onNext({ skipped: true })} disabled={busy}>
            Skip
          </button>
        )}
      </div>
    </>
  );
}
