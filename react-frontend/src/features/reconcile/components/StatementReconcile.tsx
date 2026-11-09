import {
  ArrowClockwise,
  CheckCircle,
  CircleNotch,
  FilePdf,
  Trash,
  UploadSimple,
  WarningCircle,
} from '@phosphor-icons/react';
import { useMemo, useRef, useState, type DragEvent } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { useStatementList, useStatementReconciliation, type UploadOutcome } from '../hooks/useStatements';
import type { StatementImport } from '../types/reconcile.types';
import { SectionReconcile } from './SectionReconcile';
import styles from './StatementReconcile.module.css';

const MAX_BYTES = 20 * 1024 * 1024;

function statusLabel(imp: StatementImport) {
  if (imp.status === 'processing') {
    return imp.pageCount ? `Reading pages ${imp.pagesProcessed}/${imp.pageCount}…` : 'Reading…';
  }
  if (imp.status === 'failed') return 'Failed';
  return `${imp.rowCount} rows`;
}

/**
 * Settings → Reconcile: upload bank/card statement PDFs and compare them with
 * the ledger. Matching is recomputed by the API on every load, so rows created
 * here turn into matches on the next fetch.
 */
export default function StatementReconcile() {
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedId = searchParams.get('statement');
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const openAccounts = useMemo(
    () => accounts.filter((acc) => !acc.closed && !acc.deleted && acc.id),
    [accounts],
  );

  const { imports, isLoading, error, upload, remove, retry } = useStatementList();
  const detail = useStatementReconciliation(selectedId);

  const [accountId, setAccountId] = useState(searchParams.get('account') ?? '');
  const [password, setPassword] = useState('');
  const [files, setFiles] = useState<File[]>([]);
  const [fileError, setFileError] = useState('');
  const [uploading, setUploading] = useState(false);
  const [outcomes, setOutcomes] = useState<UploadOutcome[]>([]);
  const [dragging, setDragging] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  const select = (id: string | null) => {
    const next = new URLSearchParams(searchParams);
    if (id) next.set('statement', id);
    else next.delete('statement');
    setSearchParams(next, { replace: true });
  };

  const addFiles = (list: FileList | null) => {
    if (!list) return;
    const picked = Array.from(list);
    const bad = picked.find(
      (f) => !/\.pdf$/i.test(f.name) || (f.type && f.type !== 'application/pdf'),
    );
    if (bad) {
      setFileError(`${bad.name} is not a PDF.`);
      return;
    }
    const big = picked.find((f) => f.size > MAX_BYTES);
    if (big) {
      setFileError(`${big.name} is larger than 20 MB.`);
      return;
    }
    setFileError('');
    setFiles((prev) => [...prev, ...picked.filter((f) => !prev.some((p) => p.name === f.name && p.size === f.size))]);
  };

  const onDrop = (event: DragEvent) => {
    event.preventDefault();
    setDragging(false);
    addFiles(event.dataTransfer.files);
  };

  const submit = async () => {
    if (!files.length) return;
    setUploading(true);
    setOutcomes([]);
    try {
      const results = await upload(files, accountId, password);
      setOutcomes(results);
      // keep failed files queued so a password can be added and resubmitted
      setFiles((prev) => prev.filter((f) => results.some((r) => r.fileName === f.name && !r.ok)));
      const firstOk = results.find((r) => r.ok && r.importId);
      if (firstOk?.importId) select(firstOk.importId);
      if (results.every((r) => r.ok)) setPassword('');
    } finally {
      setUploading(false);
    }
  };

  const handleRemove = async (imp: StatementImport) => {
    if (!window.confirm(`Delete ${imp.fileName}? Transactions already created stay in the ledger.`)) return;
    await remove(imp.id);
    if (selectedId === imp.id) select(null);
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <div>
          <h2>Reconcile statements</h2>
          <p>
            Upload bank or card statements as PDF. Each row is matched against your transactions; rows
            missing from Pennywise can be created for review.
          </p>
        </div>
      </div>

      <div
        className={`${styles.uploadPanel} ${dragging ? styles.dragging : ''}`}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}>
        <div className={styles.uploadFields}>
          <label className={styles.field}>
            <span>Account</span>
            <select value={accountId} onChange={(e) => setAccountId(e.target.value)}>
              <option value="">Detect from statement</option>
              {openAccounts.map((acc) => (
                <option key={acc.id} value={acc.id}>
                  {acc.name}
                </option>
              ))}
            </select>
          </label>
          <label className={styles.field}>
            <span>PDF password</span>
            <input
              type="password"
              autoComplete="off"
              placeholder="If the statement is locked"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>
          <button type="button" className={styles.secondaryButton} onClick={() => fileInput.current?.click()}>
            <FilePdf size={16} /> Choose PDFs
          </button>
          <button
            type="button"
            className={styles.primaryButton}
            disabled={!files.length || uploading}
            onClick={() => void submit()}>
            {uploading ? <CircleNotch size={16} className={styles.spin} /> : <UploadSimple size={16} />}
            Upload {files.length > 0 ? files.length : ''}
          </button>
        </div>
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
        {files.length === 0 ? (
          <p className={styles.hint}>
            Drop PDFs here. One statement may cover several accounts; each section is matched to an account by
            its number, or you can pick one.
          </p>
        ) : (
          <ul className={styles.fileList}>
            {files.map((f) => (
              <li key={`${f.name}-${f.size}`}>
                <FilePdf size={14} /> {f.name}
                <button
                  type="button"
                  aria-label={`Remove ${f.name}`}
                  onClick={() => setFiles((prev) => prev.filter((p) => p !== f))}>
                  ×
                </button>
              </li>
            ))}
          </ul>
        )}
        {fileError && <p className={styles.error}>{fileError}</p>}
        {outcomes.map((o) => (
          <p key={o.fileName} className={o.ok ? styles.hint : styles.error}>
            {o.ok ? <CheckCircle size={14} /> : <WarningCircle size={14} />} {o.fileName}
            {o.message ? `: ${o.message}` : ': queued'}
          </p>
        ))}
      </div>

      <div className={styles.layout}>
        <aside className={styles.importList} aria-label="Uploaded statements">
          {isLoading && <p className={styles.hint}>Loading…</p>}
          {error && <p className={styles.error}>{error}</p>}
          {!isLoading && imports.length === 0 && <p className={styles.hint}>No statements uploaded yet.</p>}
          {imports.map((imp) => (
            <div
              key={imp.id}
              role="button"
              tabIndex={0}
              className={`${styles.importItem} ${selectedId === imp.id ? styles.importItemActive : ''}`}
              onClick={() => select(imp.id)}
              onKeyDown={(e) => e.key === 'Enter' && select(imp.id)}>
              <div className={styles.importName}>
                {imp.status === 'processing' && <CircleNotch size={14} className={styles.spin} />}
                {imp.status === 'failed' && <WarningCircle size={14} className={styles.failedIcon} />}
                <strong title={imp.fileName}>{imp.fileName}</strong>
              </div>
              <span className={styles.importMeta}>
                {new Date(imp.createdAt).toLocaleDateString()} · {statusLabel(imp)}
              </span>
              <div className={styles.importActions}>
                {imp.status === 'failed' && (
                  <button
                    type="button"
                    title="Retry extraction"
                    onClick={(e) => {
                      e.stopPropagation();
                      void retry(imp.id);
                    }}>
                    <ArrowClockwise size={14} />
                  </button>
                )}
                <button
                  type="button"
                  title="Delete"
                  onClick={(e) => {
                    e.stopPropagation();
                    void handleRemove(imp);
                  }}>
                  <Trash size={14} />
                </button>
              </div>
            </div>
          ))}
        </aside>

        <div className={styles.detail}>
          {!selectedId && <p className={styles.hint}>Select a statement to see how it reconciles.</p>}
          {selectedId && detail.isLoading && !detail.data && <p className={styles.hint}>Loading…</p>}
          {detail.error && <p className={styles.error}>{detail.error}</p>}
          {detail.data && <StatementDetail {...detail} accounts={openAccounts} />}
        </div>
      </div>
    </div>
  );
}

function StatementDetail({
  data,
  setSectionAccount,
  createMissing,
  accounts,
}: ReturnType<typeof useStatementReconciliation> & {
  accounts: { id?: string; name: string }[];
}) {
  if (!data) return null;
  const imp = data.import;

  if (imp.status === 'processing') {
    return (
      <div className={styles.processing}>
        <CircleNotch size={20} className={styles.spin} />
        <div>
          <strong>Reading {imp.fileName}</strong>
          <p className={styles.hint}>
            Page {imp.pagesProcessed} of {imp.pageCount}. A local model reads one chunk at a time; this can take a
            few minutes.
          </p>
        </div>
      </div>
    );
  }
  if (imp.status === 'failed') {
    return (
      <div className={styles.failed}>
        <WarningCircle size={20} />
        <div>
          <strong>Could not read {imp.fileName}</strong>
          <p>{imp.error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.sections}>
      {imp.droppedRows > 0 && (
        <p className={styles.warning}>
          <WarningCircle size={14} /> {imp.droppedRows} extracted row{imp.droppedRows === 1 ? '' : 's'} had an
          unreadable date or amount and were left out. Check the balance below before trusting the totals.
        </p>
      )}
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
