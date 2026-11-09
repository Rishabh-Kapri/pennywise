import { apiClient, errorMessage } from '@/utils';
import { CircleNotch } from '@phosphor-icons/react';
import { useEffect, useState } from 'react';
import styles from './EmailBackfill.module.css';

const BACKFILL_DAYS = 14;

interface BackfillCandidate {
  externalId: string;
  sender: string;
  subject: string;
  receivedAt: string;
}

/** "Bank Alerts <alerts@bank.com>" → "Bank Alerts". */
function senderName(sender: string): string {
  return sender.replace(/\s*<[^>]*>\s*$/, '').replace(/^"|"$/g, '') || sender;
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
}

function plural(count: number): string {
  return `${count} email${count === 1 ? '' : 's'}`;
}

/**
 * Imports recent emails from a connected mailbox: all of them with
 * automatic import on, or only the ones the user ticks with it off.
 */
export function EmailBackfill({
  connectionId,
  onImported,
}: {
  connectionId: string;
  onImported: () => Promise<unknown>;
}) {
  const [candidates, setCandidates] = useState<BackfillCandidate[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [automatic, setAutomatic] = useState(true);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const [started, setStarted] = useState(false);

  useEffect(() => {
    apiClient
      .get<BackfillCandidate[]>(`mail-connections/${connectionId}/backfill`)
      .then((found) => setCandidates(found ?? []))
      .catch((err: unknown) => setLoadError(errorMessage(err, "Couldn't look through your emails.")));
  }, [connectionId]);

  if (loadError) return <p className={styles.error}>{loadError}</p>;
  if (!candidates) {
    return (
      <p className={styles.status}>
        <CircleNotch size={16} className={styles.spin} /> Looking for emails from the last {BACKFILL_DAYS} days…
      </p>
    );
  }
  if (candidates.length === 0) {
    return (
      <p className={styles.status}>
        No emails in the last {BACKFILL_DAYS} days. New ones will still be picked up as they arrive.
      </p>
    );
  }
  if (started) return <p className={styles.status}>Import started. Follow it in Settings → Jobs.</p>;

  const chosen = automatic ? candidates.map((candidate) => candidate.externalId) : [...picked];
  const allPicked = picked.size === candidates.length;

  const toggle = (id: string) => {
    const next = new Set(picked);
    if (!next.delete(id)) next.add(id);
    setPicked(next);
  };

  const importEmails = async () => {
    setImporting(true);
    setImportError(null);
    try {
      await apiClient.post<{ jobId: string }, { externalIds: string[] }>(`mail-connections/${connectionId}/backfill`, {
        externalIds: chosen,
      });
      setStarted(true);
      // Remembered on the budget so this step doesn't offer it again. Queuing the
      // same emails twice is harmless, so a failure here isn't worth showing.
      await onImported().catch(() => undefined);
    } catch (err) {
      setImportError(errorMessage(err, "Couldn't import your emails."));
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className={styles.root}>
      <label className={styles.automatic}>
        <span className={styles.automaticText}>
          <strong>Import automatically</strong>
          <small>
            {automatic
              ? `All ${plural(candidates.length)} from the last ${BACKFILL_DAYS} days will be imported.`
              : 'Choose which emails to import.'}
          </small>
        </span>
        <input
          type="checkbox"
          role="switch"
          className={styles.switch}
          checked={automatic}
          onChange={(event) => setAutomatic(event.target.checked)}
        />
      </label>

      {!automatic && (
        <>
          <button
            type="button"
            className={styles.selectAll}
            onClick={() =>
              setPicked(allPicked ? new Set() : new Set(candidates.map((candidate) => candidate.externalId)))
            }
          >
            {allPicked ? 'Clear all' : 'Select all'}
          </button>
          <ul className={styles.list}>
            {candidates.map((candidate) => (
              <li key={candidate.externalId}>
                <label className={styles.email}>
                  <input
                    type="checkbox"
                    checked={picked.has(candidate.externalId)}
                    onChange={() => toggle(candidate.externalId)}
                  />
                  <span className={styles.emailText}>
                    <span className={styles.subject}>{candidate.subject || '(no subject)'}</span>
                    <small>
                      {senderName(candidate.sender)} · {formatDate(candidate.receivedAt)}
                    </small>
                  </span>
                </label>
              </li>
            ))}
          </ul>
        </>
      )}

      {importError && <p className={styles.error}>{importError}</p>}

      <button
        type="button"
        className={styles.import}
        disabled={importing || chosen.length === 0}
        onClick={importEmails}
      >
        {importing && <CircleNotch size={16} className={styles.spin} />}
        {chosen.length === 0 ? 'Choose emails to import' : `Import ${plural(chosen.length)}`}
      </button>
    </div>
  );
}
