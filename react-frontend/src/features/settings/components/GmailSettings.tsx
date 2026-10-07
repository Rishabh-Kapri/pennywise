import { ArrowsClockwise, Envelope, Pause, Play, Plug } from '@phosphor-icons/react';
import { useGoogleLogin } from '@react-oauth/google';
import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { selectIsDemoUser } from '@/features/auth/store';
import { apiClient } from '@/utils';
import { formatUtcTimestamp } from '@/utils/date.utils';
import settingsStyles from './Settings.module.css';
import styles from './GmailSettings.module.css';

interface GmailConnection {
  providerId: string;
  oauthClientType: 'web' | 'android';
  email: string;
  name: string;
  picture?: string;
  paused: boolean;
  connected: boolean;
  status: 'active' | 'paused' | 'watch_expired' | 'needs_reconnect';
  lastGmailSync?: string;
  watchExpiresAt?: number;
}

type Action = 'pause' | 'resume' | 'sync' | 'reconnect';
interface PendingAction { key: string; action: Action }

const connectionKey = (connection: GmailConnection) =>
  `${connection.providerId}-${connection.oauthClientType}`;

const statusLabels: Record<GmailConnection['status'], string> = {
  active: 'Watch active',
  paused: 'Ingestion paused',
  watch_expired: 'Watch needs renewal',
  needs_reconnect: 'Gmail access needed',
};

interface ConnectionCardProps {
  connection: GmailConnection;
  pending: PendingAction | null;
  isDemo: boolean;
  loading: boolean;
  onAction: (connection: GmailConnection, action: Action, code?: string) => Promise<void>;
  onReconnectStart: (connection: GmailConnection) => void;
  onReconnectError: (message: string) => void;
}

function ReconnectButton({ connection, pending, onAction, onReconnectStart, onReconnectError }: ConnectionCardProps) {
  const reconnect = useGoogleLogin({
    flow: 'auth-code',
    scope: 'https://mail.google.com/',
    hint: connection.email,
    select_account: true,
    onSuccess: (response) => { void onAction(connection, 'reconnect', response.code); },
    onError: () => onReconnectError('Google authorization failed. Try reconnecting Gmail again.'),
    onNonOAuthError: (error) => onReconnectError(
      error.type === 'popup_closed'
        ? 'Reconnection was cancelled. Your Gmail connection has not changed.'
        : 'Google could not open the sign-in window. Allow pop-ups and try again.',
    ),
  });
  return (
    <button type="button" className={styles.button} disabled={pending !== null}
      onClick={() => { onReconnectStart(connection); reconnect(); }}>
      <Plug size={16} />{pending?.action === 'reconnect' && pending.key === connectionKey(connection) ? 'Reconnecting…' : 'Reconnect Gmail'}
    </button>
  );
}

function ConnectionCard(props: ConnectionCardProps) {
  const { connection, pending, isDemo, loading, onAction } = props;
  const activeAction = pending?.key === connectionKey(connection) ? pending.action : null;
  const disabled = isDemo || loading || pending !== null;
  const canReconnect = connection.oauthClientType === 'web' && !!import.meta.env.VITE_GOOGLE_CLIENT_ID;

  return (
    <article className={styles.connection} aria-label={`Gmail connection for ${connection.email} (${connection.oauthClientType})`}>
      <div className={styles.connectionHeader}>
        <div className={styles.identity}>
          <span className={styles.mailIcon}><Envelope size={22} /></span>
          <div>
            <h3>{connection.email}</h3>
            <span>{connection.oauthClientType === 'android' ? 'Android connection' : 'Web connection'}</span>
          </div>
        </div>
        <span className={styles.status} data-status={connection.status}>{statusLabels[connection.status]}</span>
      </div>

      <dl className={styles.details}>
        <div><dt>Last sync</dt><dd>{formatUtcTimestamp(connection.lastGmailSync)}</dd></div>
        <div><dt>Watch expires</dt><dd>{connection.paused ? 'Paused' : formatUtcTimestamp(connection.watchExpiresAt)}</dd></div>
      </dl>

      <p className={styles.description}>
        {connection.paused
          ? 'New email imports are paused for this mailbox. Imports already in progress may finish. Resume and sync to check for missed emails.'
          : !connection.connected
            ? 'Reconnect your Google account to allow Pennywise to import transaction emails.'
            : connection.status === 'watch_expired'
              ? 'Renew the watch to restore automatic imports, or reconnect if Google access has expired.'
              : 'Pennywise watches your inbox for new transaction emails. This status reflects the last saved watch; Google may require reconnection if access is revoked.'}
      </p>

      <div className={styles.actions}>
        {connection.paused || connection.status === 'watch_expired' ? (
          <button type="button" className={styles.primaryButton} disabled={disabled || !connection.connected}
            onClick={() => { void onAction(connection, 'resume'); }}>
            <Play size={16} />
            {activeAction === 'resume' ? 'Connecting…' : connection.paused ? 'Resume ingestion' : 'Renew watch'}
          </button>
        ) : (
          <button type="button" className={styles.button} disabled={disabled}
            onClick={() => { void onAction(connection, 'pause'); }}>
            <Pause size={16} />{activeAction === 'pause' ? 'Pausing…' : 'Pause ingestion'}
          </button>
        )}
        <button type="button" className={styles.button} disabled={disabled || connection.paused || !connection.connected}
          title={connection.paused ? 'Resume ingestion before syncing' : 'Check for emails since the saved sync position'}
          onClick={() => { void onAction(connection, 'sync'); }}>
          <ArrowsClockwise size={16} />{activeAction === 'sync' ? 'Starting sync…' : 'Sync now'}
        </button>
        {canReconnect && !isDemo && !loading ? <ReconnectButton {...props} /> : (
          <button type="button" className={styles.button} disabled
            title={isDemo ? 'Unavailable in demo mode' : connection.oauthClientType === 'android' ? 'Sign in again through the Android app to reconnect' : 'Google sign-in is not configured'}>
            <Plug size={16} />Reconnect Gmail
          </button>
        )}
      </div>
    </article>
  );
}

export function GmailSettings() {
  const isDemo = useAppSelector(selectIsDemoUser);
  const [connections, setConnections] = useState<GmailConnection[]>([]);
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const loadConnections = useCallback(async () => {
    setLoading(true);
    try {
      setConnections(await apiClient.get<GmailConnection[]>('auth/gmail'));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load Gmail connections.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void loadConnections(); }, [loadConnections]);

  const handleAction = async (connection: GmailConnection, action: Action, code?: string) => {
    setPending({ key: connectionKey(connection), action });
    setError(null);
    setMessage(null);
    try {
      await apiClient.post<unknown, { providerId: string; oauthClientType: string; code?: string }>(
        `auth/gmail/${action}`,
        { providerId: connection.providerId, oauthClientType: connection.oauthClientType, ...(code ? { code } : {}) },
      );
      setMessage(action === 'sync'
        ? 'Gmail sync started. Follow its progress in Activity.'
        : action === 'pause'
          ? 'Gmail ingestion paused for this mailbox.'
          : action === 'reconnect'
            ? connection.paused ? 'Gmail reconnected. Ingestion remains paused.' : 'Gmail reconnected and its watch renewed.'
            : 'Gmail ingestion enabled and its watch renewed. Use Sync now to check for missed emails.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not update the Gmail connection.');
    } finally {
      // Pause can succeed locally even when stopping Google's watch fails.
      await loadConnections();
      setPending(null);
    }
  };

  return (
    <div className={settingsStyles.card}>
      <div className={settingsStyles.cardHeader}>
        <h2>Gmail controls</h2>
        <button type="button" className={styles.button} disabled={loading || pending !== null}
          onClick={() => { setError(null); setMessage(null); void loadConnections(); }}>
          <ArrowsClockwise size={16} />{loading ? 'Refreshing…' : 'Refresh status'}
        </button>
      </div>
      <p className={styles.intro}>Manage automatic transaction imports from your connected Gmail accounts. These controls apply across your budgets.</p>
      {isDemo && <p className={styles.notice}>Gmail controls are unavailable in demo mode.</p>}
      {error && <p className={styles.error} role="alert">{error}</p>}
      {message && <p className={styles.notice} role="status">{message}</p>}
      {loading && connections.length === 0 ? (
        <p className={settingsStyles.empty} role="status">Loading Gmail connections…</p>
      ) : connections.length === 0 ? (
        <p className={settingsStyles.empty}>{error ? 'Refresh status to retry loading your Gmail connections.' : 'No Gmail accounts connected. Sign in with Google to connect your mailbox.'}</p>
      ) : (
        <div className={styles.connectionList}>
          {connections.map(connection => (
            <ConnectionCard key={connectionKey(connection)} connection={connection} pending={pending} isDemo={isDemo} loading={loading}
              onAction={handleAction}
              onReconnectStart={connection => { setError(null); setMessage(null); setPending({ key: connectionKey(connection), action: 'reconnect' }); }}
              onReconnectError={message => { setError(message); setPending(null); }} />
          ))}
        </div>
      )}
      <div className={styles.footer}>
        <span>Sync checks emails since the saved sync position. If that position is too old, Gmail may require recovery.</span>
        <Link to="/settings?section=activity">View import activity →</Link>
      </div>
    </div>
  );
}
