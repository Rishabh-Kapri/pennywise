import { ArrowSquareOut, CircleNotch, X } from '@phosphor-icons/react';
import { useEffect, useState } from 'react';
import { apiClient, errorMessage } from '@/utils';
import styles from './StatementReconcile.module.css';

/** A page to turn to, 1-based; `at` makes asking for the same page again count. */
export interface PdfPageRequest {
  page: number;
  at: number;
}

/**
 * The statement's PDF beside its rows, to check what was read against it.
 * The file needs auth headers, so it's fetched once as a blob; the browser's
 * own viewer renders it (and asks for the password of a locked statement).
 * Rows that know their page turn it with `#page=N`.
 */
export function StatementPdfPanel({
  statementId,
  request,
  onClose,
}: {
  statementId: string;
  request: PdfPageRequest | null;
  onClose: () => void;
}) {
  const [url, setUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let blobUrl: string | null = null;
    let cancelled = false;
    apiClient
      .fetchBlobUrl(`statements/${statementId}/file`)
      .then((u) => {
        blobUrl = u;
        if (cancelled) URL.revokeObjectURL(u);
        else setUrl(u);
      })
      .catch((err) => !cancelled && setError(errorMessage(err, "Couldn't load the PDF")));
    return () => {
      cancelled = true;
      if (blobUrl) URL.revokeObjectURL(blobUrl);
    };
  }, [statementId]);

  const src = url ? `${url}#page=${request?.page ?? 1}` : undefined;

  return (
    <aside className={styles.pdfPanel} aria-label="Statement PDF">
      <div className={styles.pdfBar}>
        <span>{request ? `Page ${request.page}` : 'Statement PDF'}</span>
        <span className={styles.spacer} />
        {url && (
          <a className={styles.iconButton} href={url} target="_blank" rel="noreferrer" title="Open in a new tab">
            <ArrowSquareOut size={17} />
          </a>
        )}
        <button
          type="button"
          className={styles.iconButton}
          title="Close the PDF"
          aria-label="Close the PDF"
          onClick={onClose}
        >
          <X size={17} />
        </button>
      </div>
      {error ? (
        <p className={styles.error}>{error}</p>
      ) : !src ? (
        <div className={styles.pdfLoading}>
          <CircleNotch size={24} className={styles.spin} />
        </div>
      ) : (
        <iframe key={url} className={styles.pdfFrame} src={src} title="Statement PDF" />
      )}
    </aside>
  );
}
