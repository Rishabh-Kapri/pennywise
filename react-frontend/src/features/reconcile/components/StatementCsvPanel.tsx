import { CircleNotch, X } from '@phosphor-icons/react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { apiClient, errorMessage } from '@/utils';
import { parseExport } from '../parseExport';
import styles from './StatementReconcile.module.css';

/** A line to show, 1-based; `at` makes asking for the same line again count. */
export interface CsvLineRequest {
  line: number;
  at: number;
}

/**
 * The statement's CSV beside its rows, as a table with the reader's line
 * numbers. Rows that know their line scroll to it and light it up.
 */
export function StatementCsvPanel({
  statementId,
  request,
  rowLines,
  onClose,
}: {
  statementId: string;
  request: CsvLineRequest | null;
  /** lines the reader took as transactions; the rest is the bank's summary */
  rowLines: Set<number>;
  onClose: () => void;
}) {
  const [text, setText] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const bodyRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    let blobUrl: string | null = null;
    apiClient
      .fetchBlobUrl(`statements/${statementId}/file`)
      .then(async (u) => {
        blobUrl = u;
        const body = await (await fetch(u)).text();
        if (!cancelled) setText(body);
      })
      .catch((err) => !cancelled && setError(errorMessage(err, "Couldn't load the CSV")))
      .finally(() => blobUrl && URL.revokeObjectURL(blobUrl));
    return () => {
      cancelled = true;
    };
  }, [statementId]);

  const records = useMemo(() => (text === null ? [] : parseExport(text)), [text]);
  const width = useMemo(() => records.reduce((max, r) => Math.max(max, r.length), 0), [records]);

  useEffect(() => {
    const body = bodyRef.current;
    if (!request || !body) return;
    // scroll the panel only: scrollIntoView would move the page too
    const row = body.querySelector<HTMLElement>(`[data-line="${request.line}"]`);
    if (row) body.scrollTo({ top: row.offsetTop - body.clientHeight / 2 + row.offsetHeight / 2, behavior: 'smooth' });
  }, [request, records]);

  return (
    <aside className={styles.pdfPanel} aria-label="Statement CSV">
      <div className={styles.pdfBar}>
        <span>{request ? `Line ${request.line}` : 'Statement CSV'}</span>
        <span className={styles.spacer} />
        <button
          type="button"
          className={styles.iconButton}
          title="Close the CSV"
          aria-label="Close the CSV"
          onClick={onClose}
        >
          <X size={17} />
        </button>
      </div>
      {error ? (
        <p className={styles.error}>{error}</p>
      ) : text === null ? (
        <div className={styles.pdfLoading}>
          <CircleNotch size={24} className={styles.spin} />
        </div>
      ) : (
        <div ref={bodyRef} className={styles.csvBody}>
          <table className={styles.csvTable}>
            <tbody>
              {records.map((record, i) => {
                const line = i + 1;
                const classes = [rowLines.has(line) ? '' : styles.csvAside, request?.line === line ? styles.csvHit : '']
                  .filter(Boolean)
                  .join(' ');
                return (
                  <tr key={line} data-line={line} className={classes || undefined}>
                    <th scope="row">{line}</th>
                    {Array.from({ length: width }, (_, c) => (
                      <td key={c} title={record[c] && record[c].length > 32 ? record[c] : undefined}>
                        {record[c] ?? ''}
                      </td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </aside>
  );
}
