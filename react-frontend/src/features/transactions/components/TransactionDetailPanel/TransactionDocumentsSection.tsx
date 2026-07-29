import { useRef } from 'react';
import { FilePdfIcon, PaperclipIcon, PlusIcon, TrashIcon } from '@phosphor-icons/react';
import { useTransactionDocuments } from '../../hooks/useTransactionDocuments';
import type { Transaction, TransactionDocument } from '../../types/transaction.types';
import styles from './TransactionDetailPanel.module.css';

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function DocumentTile({
  doc,
  previewUrl,
  onOpen,
  onRemove,
}: {
  doc: TransactionDocument;
  previewUrl?: string;
  onOpen: () => void;
  onRemove: () => void;
}) {
  return (
    <div className={styles.documentTile}>
      <button type="button" className={styles.documentPreview} onClick={onOpen} title={doc.fileName}>
        {previewUrl ? (
          <img src={previewUrl} alt={doc.fileName} className={styles.documentThumb} />
        ) : (
          <span className={styles.documentIcon}>
            {doc.mimeType === 'application/pdf' ? <FilePdfIcon size={28} /> : <PaperclipIcon size={28} />}
          </span>
        )}
      </button>
      <div className={styles.documentMeta}>
        <span className={styles.documentName} title={doc.fileName}>
          {doc.fileName}
        </span>
        <span className={styles.documentSize}>{formatSize(doc.sizeBytes)}</span>
      </div>
      <button
        type="button"
        className={styles.documentDeleteBtn}
        onClick={onRemove}
        aria-label={`Delete ${doc.fileName}`}>
        <TrashIcon size={14} />
      </button>
    </div>
  );
}

export function TransactionDocumentsSection({ txn }: { txn: Transaction }) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const pdfInputRef = useRef<HTMLInputElement | null>(null);
  const {
    documents,
    previewUrls,
    isLoading,
    isUploading,
    uploadProgress,
    error,
    upload,
    uploadAsPdf,
    remove,
    openDocument,
  } = useTransactionDocuments(txn.id || undefined);

  if (!txn.id) return null;

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    if (files.length > 0) void upload(files);
    event.target.value = '';
  };

  const handlePdfFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    if (files.length > 0) void uploadAsPdf(files);
    event.target.value = '';
  };

  return (
    <section className={styles.documentsSection}>
      <span className={styles.metaLabel}>
        <PaperclipIcon size={18} />
        <span>Receipts &amp; Documents</span>
      </span>

      <div className={styles.documentGrid}>
        {documents.map((doc) => (
          <DocumentTile
            key={doc.id}
            doc={doc}
            previewUrl={previewUrls[doc.id]}
            onOpen={() => void openDocument(doc)}
            onRemove={() => void remove(doc.id)}
          />
        ))}

        <button
          type="button"
          className={styles.documentAddBtn}
          onClick={() => inputRef.current?.click()}
          disabled={isUploading}>
          <PlusIcon size={18} />
          <span>
            {isUploading
              ? uploadProgress
                ? `${uploadProgress.done}/${uploadProgress.total}`
                : 'Uploading…'
              : 'Add'}
          </span>
        </button>
      </div>

      <div className={styles.locationActions}>
        <button
          type="button"
          className={styles.locationBtnSecondary}
          onClick={() => pdfInputRef.current?.click()}
          disabled={isUploading}
          title="Combine several page images into a single PDF">
          <FilePdfIcon size={14} />
          Combine images to PDF
        </button>
      </div>

      {isLoading && documents.length === 0 && <span className={styles.emptyValue}>Loading documents…</span>}
      {error && <span className={styles.errorText}>{error}</span>}

      <input
        ref={pdfInputRef}
        type="file"
        multiple
        accept="image/jpeg,image/png"
        className={styles.hiddenFileInput}
        onChange={handlePdfFileChange}
      />

      <input
        ref={inputRef}
        type="file"
        multiple
        accept="image/jpeg,image/png,image/webp,image/heic,application/pdf"
        className={styles.hiddenFileInput}
        onChange={handleFileChange}
      />
    </section>
  );
}
