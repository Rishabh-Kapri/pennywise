import { useAppSelector } from '@/app/hooks';
import { selectIsDemoUser } from '@/features/auth/store/authSlice';
import { useMailConnections, type MailConnection } from '@/features/mail/useMailConnections';
import { CheckCircle, CircleNotch, EnvelopeSimple, PauseCircle, Warning } from '@phosphor-icons/react';
import { useState } from 'react';
import styles from './MailSettings.module.css';

function statusText(connection: MailConnection): string {
  if (connection.status === 'needs_reconnect') return 'Access ended. Reconnect to resume.';
  if (connection.pausedAt) return "Paused. Mail that arrives now won't be imported.";
  if (connection.status === 'error') return "Last sync didn't work. Reconnect to retry.";
  if (!connection.lastSyncAt) return 'Connected, waiting for new mail';
  const synced = new Date(connection.lastSyncAt).toLocaleString(undefined, {
    day: 'numeric',
    month: 'short',
    hour: 'numeric',
    minute: '2-digit',
  });
  return `Last synced ${synced}`;
}

function StatusIcon({ connection }: { connection: MailConnection }) {
  if (connection.status !== 'active') return <Warning size={20} weight="fill" className={styles.warn} />;
  if (connection.pausedAt) return <PauseCircle size={20} weight="fill" className={styles.paused} />;
  return <CheckCircle size={20} weight="fill" className={styles.ok} />;
}

interface MailRowProps {
  connection: MailConnection;
  connecting: boolean;
  readOnly: boolean;
  onReconnect: () => void;
  onSetPaused: (paused: boolean) => Promise<void>;
  onDisconnect: () => Promise<void>;
}

function MailRow({ connection, connecting, readOnly, onReconnect, onSetPaused, onDisconnect }: MailRowProps) {
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    try {
      await action();
    } finally {
      setBusy(false);
    }
  };

  const paused = Boolean(connection.pausedAt);
  const canReconnect = connection.status !== 'active';
  const canPause = connection.status !== 'needs_reconnect';

  return (
    <li className={styles.row}>
      <StatusIcon connection={connection} />
      <span className={styles.rowText}>
        <span className={styles.account}>{connection.account}</span>
        <span className={styles.muted}>
          {confirming ? 'Disconnect from this budget? Imported transactions stay.' : statusText(connection)}
        </span>
      </span>
      {!readOnly && (
        <div className={styles.actions}>
          {confirming ? (
            <>
              <button type="button" className={styles.action} disabled={busy} onClick={() => setConfirming(false)}>
                Keep
              </button>
              <button
                type="button"
                className={`${styles.action} ${styles.danger}`}
                disabled={busy}
                onClick={() => void run(onDisconnect)}>
                Disconnect
              </button>
            </>
          ) : (
            <>
              {canReconnect && (
                <button
                  type="button"
                  className={`${styles.action} ${styles.emphasis}`}
                  disabled={connecting}
                  onClick={onReconnect}>
                  Reconnect
                </button>
              )}
              {canPause && (
                <button
                  type="button"
                  className={styles.action}
                  disabled={busy}
                  onClick={() => void run(() => onSetPaused(!paused))}>
                  {paused ? 'Resume' : 'Pause'}
                </button>
              )}
              <button type="button" className={styles.action} disabled={busy} onClick={() => setConfirming(true)}>
                Disconnect
              </button>
            </>
          )}
        </div>
      )}
    </li>
  );
}

export function MailSettings() {
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const { connections, loadError, connecting, connect, setPaused, disconnect } = useMailConnections();

  return (
    <div className={styles.page}>
      {loadError && <p className={styles.notice}>{loadError}</p>}
      {!connections && !loadError && <p className={styles.muted}>Loading…</p>}

      {connections?.length === 0 && (
        <div className={styles.empty}>
          <EnvelopeSimple size={28} />
          <h3>Turn bank emails into transactions</h3>
          <p>Connect the Gmail account that receives your bank alerts. Pennywise can only read mail.</p>
        </div>
      )}

      {connections && connections.length > 0 && (
        <ul className={styles.list}>
          {connections.map((connection) => (
            <MailRow
              key={connection.id}
              connection={connection}
              connecting={connecting}
              readOnly={isDemoUser}
              onReconnect={() => connect(connection.account)}
              onSetPaused={(paused) => setPaused(connection, paused)}
              onDisconnect={() => disconnect(connection)}
            />
          ))}
        </ul>
      )}

      {connections && !isDemoUser && (
        <div>
          <button type="button" className={styles.connect} disabled={connecting} onClick={() => connect()}>
            {connecting ? <CircleNotch size={18} className={styles.spin} /> : <EnvelopeSimple size={18} />}
            {connections.length > 0 ? 'Connect another Gmail' : 'Connect Gmail'}
          </button>
        </div>
      )}
      {isDemoUser && <p className={styles.muted}>Email sync isn't available in the demo.</p>}
    </div>
  );
}
