import {
  ArrowClockwise,
  ArrowLeft,
  ArrowSquareOut,
  CaretRight,
  CircleNotch,
  FilePdf,
  Trash,
  UploadSimple,
  WarningCircle,
  X,
} from '@phosphor-icons/react';
import { useMemo, useRef, useState, type DragEvent } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { errorMessage, toast } from '@/utils';
import {
  openStatementPdf,
  statementProgressLabel,
  useStatementList,
  useStatementReconciliation,
} from '../hooks/useStatements';
import type { StatementImport } from '../types/reconcile.types';
import { SectionReconcile } from './SectionReconcile';
import styles from './StatementReconcile.module.css';

const MAX_BYTES = 20 * 1024 * 1024;

const longDate = (iso: string) =>
  new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });

const fileSize = (bytes: number) =>
  bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`;

/** 0..100 while pages are being read, or null when there's no count yet. */
function progressPercent(imp: StatementImport): number | null {
  if (!imp.pageCount) return null;
  return Math.min(100, Math.round((imp.pagesProcessed / imp.pageCount) * 100));
}

function ProgressBar({ imp }: { imp: StatementImport }) {
  const percent = progressPercent(imp);
  return (
    <div className={styles.progressTrack} aria-hidden>
      <div
        className={percent === null ? styles.progressIndeterminate : styles.progressFill}
        style={percent === null ? undefined : { width: `${Math.max(percent, 4)}%` }}
      />
    </div>
  );
}

/**
 * Settings → Reconcile: upload bank/card statement PDFs and compare them with
 * the ledger. Two views: the list of statements (with upload), and one
 * statement (`&statement=<id>`). Matching is recomputed by the API on every
 * load, so rows created here turn into matches on the next fetch.
 *
 * A credit card's Statements tab (Accounts) renders this same component with
 * the card's accountId: uploads go to that card and only its statements show.
 */
export default function StatementReconcile({ accountId: cardId }: { accountId?: string } = {}) {
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedId = searchParams.get('statement');
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const openAccounts = useMemo(() => accounts.filter((acc) => !acc.closed && !acc.deleted && acc.id), [accounts]);

  const { imports: allImports, isLoading, error, upload, remove, retry } = useStatementList();
  const imports = useMemo(
    // a statement read for this card, or uploaded from it and still being read
    () =>
      cardId
        ? allImports.filter((imp) =>
            imp.accountIds?.length ? imp.accountIds.includes(cardId) : imp.defaultAccountId === cardId,
          )
        : allImports,
    [allImports, cardId],
  );
  const detail = useStatementReconciliation(selectedId);

  const [accountId, setAccountId] = useState(cardId ?? searchParams.get('account') ?? '');
  const [password, setPassword] = useState('');
  const [files, setFiles] = useState<File[]>([]);
  const [uploading, setUploading] = useState(false);
  const [dragging, setDragging] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  // pushed, not replaced, so the browser's Back returns to the list
  const select = (id: string | null) => {
    const next = new URLSearchParams(searchParams);
    if (id) next.set('statement', id);
    else next.delete('statement');
    setSearchParams(next);
  };

  const addFiles = (list: FileList | null) => {
    if (!list) return;
    const picked = Array.from(list);
    const bad = picked.find((f) => !/\.pdf$/i.test(f.name) || (f.type && f.type !== 'application/pdf'));
    if (bad) {
      toast.error('Only PDFs can be uploaded', { description: `${bad.name} is not a PDF.` });
      return;
    }
    const big = picked.find((f) => f.size > MAX_BYTES);
    if (big) {
      toast.error('File too large', { description: `${big.name} is larger than 20 MB.` });
      return;
    }
    setFiles((prev) => [...prev, ...picked.filter((f) => !prev.some((p) => p.name === f.name && p.size === f.size))]);
  };

  const onDrop = (event: DragEvent) => {
    event.preventDefault();
    setDragging(false);
    addFiles(event.dataTransfer.files);
  };

  const cancelUpload = () => {
    setFiles([]);
    setPassword('');
  };

  const submit = async () => {
    if (!files.length) return;
    setUploading(true);
    try {
      const results = await upload(files, accountId, password);
      for (const r of results) {
        if (!r.ok) toast.error(`Couldn't upload ${r.fileName}`, { description: r.message });
        else if (r.message) toast.info(r.fileName, { description: r.message });
      }
      const queued = results.filter((r) => r.ok && !r.message).length;
      if (queued) {
        toast.success(queued === 1 ? 'Statement uploaded' : `${queued} statements uploaded`, {
          description: 'Reading the pages now. You can leave this screen.',
        });
      }
      // keep failed files staged so a password can be added and resubmitted
      setFiles((prev) => prev.filter((f) => results.some((r) => r.fileName === f.name && !r.ok)));
      if (results.every((r) => r.ok)) setPassword('');
      // one statement: go and watch it; several: stay on the list
      if (results.length === 1 && results[0].ok && results[0].importId) select(results[0].importId);
    } finally {
      setUploading(false);
    }
  };

  const handleRemove = async (imp: StatementImport) => {
    if (!window.confirm(`Delete ${imp.fileName}? Transactions already created stay in the ledger.`)) return;
    try {
      await remove(imp.id);
      if (selectedId === imp.id) select(null);
    } catch (err) {
      toast.error("Couldn't delete the statement", { description: errorMessage(err, 'Try again.') });
    }
  };

  const handleRetry = async (imp: StatementImport) => {
    try {
      await retry(imp.id);
      await detail.reload();
    } catch (err) {
      toast.error("Couldn't retry", { description: errorMessage(err, 'Upload the statement again instead.') });
    }
  };

  const viewPdf = async (imp: StatementImport) => {
    try {
      await openStatementPdf(imp.id);
    } catch (err) {
      toast.error("Couldn't open the PDF", { description: errorMessage(err, 'Try again.') });
    }
  };

  // ── One statement ──────────────────────────────────────────────
  if (selectedId) {
    const current = detail.data?.import ?? imports.find((imp) => imp.id === selectedId);
    return (
      <div className={styles.page}>
        <button type="button" className={styles.back} onClick={() => select(null)}>
          <ArrowLeft size={16} /> All statements
        </button>

        {current && (
          <header className={styles.detailHeader}>
            <div className={styles.detailTitle}>
              <h2 title={current.fileName}>{current.fileName}</h2>
              <p>Uploaded {longDate(current.createdAt)}</p>
            </div>
            <div className={styles.detailActions}>
              {current.hasFile && (
                <button type="button" className={styles.secondaryButton} onClick={() => void viewPdf(current)}>
                  <ArrowSquareOut size={17} /> View PDF
                </button>
              )}
              <button
                type="button"
                className={styles.iconButton}
                title="Delete statement"
                aria-label="Delete statement"
                onClick={() => void handleRemove(current)}>
                <Trash size={18} />
              </button>
            </div>
          </header>
        )}

        {detail.isLoading && !detail.data && <p className={styles.muted}>Loading…</p>}
        {detail.error && <p className={styles.error}>{detail.error}</p>}
        {detail.data && (
          <StatementDetail
            {...detail}
            accounts={openAccounts}
            onRetry={() => void handleRetry(detail.data!.import)}
            onDelete={() => void handleRemove(detail.data!.import)}
          />
        )}
      </div>
    );
  }

  // ── All statements ─────────────────────────────────────────────
  return (
    <div className={styles.page}>
      {!cardId && (
        <header className={styles.header}>
          <h2>Reconcile statements</h2>
          <p>
            Upload a bank or card statement. Pennywise reads it and shows which rows already match your transactions
            and which are missing.
          </p>
        </header>
      )}

      <input
        ref={fileInput}
        type="file"
        multiple
        accept=".pdf,application/pdf"
        className={styles.hiddenInput}
        onChange={(e) => {
          addFiles(e.target.files);
          e.target.value = '';
        }}
      />

      <div
        className={`${styles.dropzone} ${dragging ? styles.dragging : ''} ${files.length ? styles.dropzoneStaged : ''}`}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}>
        {files.length === 0 ? (
          <button type="button" className={styles.dropTarget} onClick={() => fileInput.current?.click()}>
            <span className={styles.dropIcon}>
              <UploadSimple size={26} />
            </span>
            <strong>Drop a statement PDF here</strong>
            <span>
              or <u>choose a file</u> · up to 20 MB · locked PDFs are fine
            </span>
          </button>
        ) : (
          <div className={styles.staged}>
            <ul className={styles.fileList}>
              {files.map((f) => (
                <li key={`${f.name}-${f.size}`}>
                  <span className={styles.fileIcon}>
                    <FilePdf size={20} />
                  </span>
                  <div className={styles.fileText}>
                    <strong title={f.name}>{f.name}</strong>
                    <span>{fileSize(f.size)}</span>
                  </div>
                  <button
                    type="button"
                    className={styles.iconButton}
                    aria-label={`Remove ${f.name}`}
                    onClick={() => setFiles((prev) => prev.filter((p) => p !== f))}>
                    <X size={16} />
                  </button>
                </li>
              ))}
            </ul>
            <div className={`${styles.stagedFields} ${cardId ? styles.stagedFieldsSingle : ''}`}>
              {!cardId && (
                <label className={styles.field}>
                  <span>Account</span>
                  <select value={accountId} onChange={(e) => setAccountId(e.target.value)}>
                    <option value="">I'll choose later</option>
                    {openAccounts.map((acc) => (
                      <option key={acc.id} value={acc.id}>
                        {acc.name}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <label className={styles.field}>
                <span>PDF password</span>
                <input
                  type="password"
                  autoComplete="off"
                  placeholder="Only if it's locked"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </label>
            </div>
            <div className={styles.stagedActions}>
              <button type="button" className={styles.textButton} onClick={() => fileInput.current?.click()}>
                + Add another file
              </button>
              <span className={styles.spacer} />
              <button type="button" className={styles.ghostButton} disabled={uploading} onClick={cancelUpload}>
                Cancel
              </button>
              <button type="button" className={styles.primaryButton} disabled={uploading} onClick={() => void submit()}>
                {uploading ? <CircleNotch size={18} className={styles.spin} /> : <UploadSimple size={18} />}
                {files.length === 1 ? 'Upload statement' : `Upload ${files.length} statements`}
              </button>
            </div>
          </div>
        )}
      </div>

      <section className={styles.listSection} aria-label="Uploaded statements">
        {imports.length > 0 && <h3>{cardId ? "This card's statements" : 'Your statements'}</h3>}
        {isLoading && <p className={styles.muted}>Loading…</p>}
        {error && <p className={styles.error}>{error}</p>}
        {imports.length > 0 && (
          <ul className={styles.importList}>
            {imports.map((imp) => (
              <li key={imp.id}>
                <div
                  role="button"
                  tabIndex={0}
                  className={styles.importRow}
                  onClick={() => select(imp.id)}
                  onKeyDown={(e) => e.key === 'Enter' && select(imp.id)}>
                  <span className={`${styles.fileIcon} ${imp.status === 'failed' ? styles.fileIconFailed : ''}`}>
                    {imp.status === 'failed' ? <WarningCircle size={20} /> : <FilePdf size={20} />}
                  </span>
                  <div className={styles.fileText}>
                    <strong title={imp.fileName}>{imp.fileName}</strong>
                    {imp.status === 'processing' ? (
                      <>
                        <span>{statementProgressLabel(imp)}</span>
                        <ProgressBar imp={imp} />
                      </>
                    ) : imp.status === 'failed' ? (
                      <span className={styles.failedText}>Couldn't be read</span>
                    ) : (
                      <span>
                        {imp.rowCount} rows · uploaded {longDate(imp.createdAt)}
                      </span>
                    )}
                  </div>
                  <div className={styles.rowActions}>
                    {imp.status === 'failed' && imp.jobId && (
                      <button
                        type="button"
                        className={styles.iconButton}
                        title="Retry reading"
                        aria-label={`Retry reading ${imp.fileName}`}
                        onClick={(e) => {
                          e.stopPropagation();
                          void handleRetry(imp);
                        }}>
                        <ArrowClockwise size={18} />
                      </button>
                    )}
                    <button
                      type="button"
                      className={styles.iconButton}
                      title="Delete"
                      aria-label={`Delete ${imp.fileName}`}
                      onClick={(e) => {
                        e.stopPropagation();
                        void handleRemove(imp);
                      }}>
                      <Trash size={18} />
                    </button>
                  </div>
                  <CaretRight size={18} className={styles.chevron} />
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function StatementDetail({
  data,
  setSectionAccount,
  createMissing,
  accounts,
  onRetry,
  onDelete,
}: ReturnType<typeof useStatementReconciliation> & {
  accounts: { id?: string; name: string }[];
  onRetry: () => void;
  onDelete: () => void;
}) {
  if (!data) return null;
  const imp = data.import;

  if (imp.status === 'processing') {
    const percent = progressPercent(imp);
    return (
      <div className={styles.stateCard}>
        <CircleNotch size={32} className={`${styles.spin} ${styles.stateIcon}`} />
        <h3>Reading your statement{percent !== null ? ` · ${percent}%` : ''}</h3>
        <ProgressBar imp={imp} />
        <p>{statementProgressLabel(imp)}</p>
        <p className={styles.muted}>
          This can take a few minutes. You can leave this screen and come back; reading carries on.
        </p>
      </div>
    );
  }

  if (imp.status === 'failed') {
    return (
      <div className={styles.stateCard}>
        <WarningCircle size={32} className={`${styles.stateIcon} ${styles.stateIconFailed}`} />
        <h3>This statement couldn't be read</h3>
        {imp.error && <p className={styles.errorDetail}>{imp.error}</p>}
        <p className={styles.muted}>
          If the PDF is locked with a password, delete it and upload it again with the password.
        </p>
        <div className={styles.stateActions}>
          {imp.jobId && (
            <button type="button" className={styles.primaryButton} onClick={onRetry}>
              <ArrowClockwise size={17} /> Try again
            </button>
          )}
          <button type="button" className={styles.secondaryButton} onClick={onDelete}>
            <Trash size={17} /> Delete
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.sections}>
      {imp.droppedRows > 0 && (
        <p className={styles.warningBanner}>
          <WarningCircle size={18} />
          <span>
            {imp.droppedRows} row{imp.droppedRows === 1 ? '' : 's'} had an unreadable date or amount and{' '}
            {imp.droppedRows === 1 ? 'was' : 'were'} left out. Check the balance before trusting the totals.
          </span>
        </p>
      )}
      {data.sections.length === 0 && <p className={styles.muted}>No transactions were found in this statement.</p>}
      {data.sections.map((section) => (
        <SectionReconcile
          key={section.index}
          section={section}
          accounts={accounts}
          onAccountChange={(accountId) => setSectionAccount(section.index, accountId)}
          onCreate={createMissing}
        />
      ))}
    </div>
  );
}
