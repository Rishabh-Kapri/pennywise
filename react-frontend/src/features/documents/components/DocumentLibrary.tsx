import {
  ArrowSquareOutIcon,
  ArrowClockwiseIcon,
  FileIcon,
  FilePdfIcon,
  ImageIcon,
  MagnifyingGlassIcon,
  ReceiptIcon,
  TrashIcon,
  XIcon,
} from '@phosphor-icons/react';
import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import { useDocumentLibrary } from '../hooks/useDocumentLibrary';
import {
  EMPTY_DOCUMENT_FILTERS,
  type DocumentFilters,
  type DocumentKind,
  type DocumentListItem,
} from '../types/document.types';
import styles from './DocumentLibrary.module.css';

const KIND_TABS: { id: DocumentKind; label: string }[] = [
  { id: '', label: 'All' },
  { id: 'image', label: 'Photos' },
  { id: 'pdf', label: 'PDFs' },
];

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(date: string): string {
  const parsed = new Date(date);
  if (Number.isNaN(parsed.getTime())) return date;
  return parsed.toLocaleDateString('en-IN', { day: 'numeric', month: 'short', year: 'numeric' });
}

function DocumentTile({
  doc,
  thumbnail,
  onOpen,
  onDelete,
}: {
  doc: DocumentListItem;
  thumbnail?: string;
  onOpen: () => void;
  onDelete: () => void;
}) {
  const [isConfirmingDelete, setIsConfirmingDelete] = useState(false);
  const isPdf = doc.mimeType === 'application/pdf';

  return (
    <li className={styles.tile}>
      <button
        type="button"
        className={styles.preview}
        onClick={onOpen}
        title={`Open ${doc.fileName}`}>
        {thumbnail ? (
          <img src={thumbnail} alt={doc.fileName} className={styles.thumb} loading="lazy" />
        ) : (
          <span className={styles.previewIcon}>
            {isPdf ? <FilePdfIcon size={30} /> : <FileIcon size={30} />}
          </span>
        )}
        <span className={styles.previewHover}>
          <ArrowSquareOutIcon size={18} />
        </span>
      </button>

      <div className={styles.meta}>
        <strong className={styles.payee} title={doc.payeeName ?? doc.fileName}>
          {doc.payeeName ?? doc.fileName}
        </strong>
        <span className={styles.metaLine}>
          {formatDate(doc.transactionDate)} · {getCurrencyLocaleString(Math.abs(doc.transactionAmount))}
        </span>
        <span className={styles.metaFaint} title={doc.fileName}>
          {doc.fileName} · {formatSize(doc.sizeBytes)}
        </span>
      </div>

      <div className={styles.tileActions}>
        <Link
          to={`/transactions/${doc.transactionId}`}
          className={styles.tileLink}
          title="Go to transaction">
          <ReceiptIcon size={15} />
        </Link>
        {isConfirmingDelete ? (
          <>
            <button
              type="button"
              className={`${styles.iconButton} ${styles.iconButtonDanger}`}
              onClick={() => {
                setIsConfirmingDelete(false);
                onDelete();
              }}
              title="Confirm delete">
              <TrashIcon size={15} weight="fill" />
            </button>
            <button
              type="button"
              className={styles.iconButton}
              onClick={() => setIsConfirmingDelete(false)}
              title="Cancel">
              <XIcon size={15} />
            </button>
          </>
        ) : (
          <button
            type="button"
            className={styles.iconButton}
            onClick={() => setIsConfirmingDelete(true)}
            title="Delete document">
            <TrashIcon size={15} />
          </button>
        )}
      </div>
    </li>
  );
}

/**
 * Every receipt in the budget in one grid, filterable by text, kind and
 * transaction date. Each tile links back to the transaction it belongs to.
 */
export default function DocumentLibrary() {
  const [filters, setFilters] = useState<DocumentFilters>(EMPTY_DOCUMENT_FILTERS);
  const [searchInput, setSearchInput] = useState('');

  // debounce so typing doesn't fire a request per keystroke
  useEffect(() => {
    const timer = setTimeout(() => {
      setFilters((prev) => (prev.search === searchInput ? prev : { ...prev, search: searchInput }));
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const {
    documents,
    thumbnails,
    total,
    hasMore,
    isLoading,
    isLoadingMore,
    error,
    loadMore,
    refresh,
    remove,
    open,
  } = useDocumentLibrary(filters);

  const hasActiveFilters = useMemo(
    () => Boolean(filters.search || filters.kind || filters.startDate || filters.endDate),
    [filters],
  );

  const clearFilters = () => {
    setSearchInput('');
    setFilters(EMPTY_DOCUMENT_FILTERS);
  };

  return (
    <section className={styles.container}>
      <header className={styles.header}>
        <div>
          <h2>Documents</h2>
          <p>
            {isLoading
              ? 'Loading receipts…'
              : `${total} ${total === 1 ? 'receipt' : 'receipts'} across this budget`}
          </p>
        </div>
        <button type="button" className={styles.refreshButton} onClick={refresh} title="Refresh">
          <ArrowClockwiseIcon size={16} />
        </button>
      </header>

      <div className={styles.filters}>
        <div className={styles.searchBox}>
          <MagnifyingGlassIcon size={16} className={styles.searchIcon} />
          <input
            type="search"
            value={searchInput}
            placeholder="Search by payee or file name"
            onChange={(event) => setSearchInput(event.target.value)}
            className={styles.searchInput}
          />
        </div>

        <div className={styles.kindTabs} role="tablist" aria-label="Document type">
          {KIND_TABS.map((tab) => (
            <button
              key={tab.id || 'all'}
              type="button"
              role="tab"
              aria-selected={filters.kind === tab.id}
              className={`${styles.kindTab} ${filters.kind === tab.id ? styles.kindTabActive : ''}`}
              onClick={() => setFilters((prev) => ({ ...prev, kind: tab.id }))}>
              {tab.label}
            </button>
          ))}
        </div>

        <div className={styles.dateRange}>
          <input
            type="date"
            value={filters.startDate}
            aria-label="From date"
            max={filters.endDate || undefined}
            onChange={(event) => setFilters((prev) => ({ ...prev, startDate: event.target.value }))}
            className={styles.dateInput}
          />
          <span className={styles.dateDash}>–</span>
          <input
            type="date"
            value={filters.endDate}
            aria-label="To date"
            min={filters.startDate || undefined}
            onChange={(event) => setFilters((prev) => ({ ...prev, endDate: event.target.value }))}
            className={styles.dateInput}
          />
        </div>

        {hasActiveFilters && (
          <button type="button" className={styles.clearButton} onClick={clearFilters}>
            <XIcon size={14} /> Clear
          </button>
        )}
      </div>

      {error && <p className={styles.error}>{error}</p>}

      {isLoading ? (
        <div className={styles.grid} aria-hidden>
          {Array.from({ length: 8 }).map((_, index) => (
            <div key={index} className={styles.skeleton} />
          ))}
        </div>
      ) : documents.length === 0 ? (
        <div className={styles.empty}>
          <ImageIcon size={34} />
          <p>
            {hasActiveFilters
              ? 'No documents match these filters.'
              : 'No receipts yet. Attach one from a transaction, or scan a bill from the mobile app.'}
          </p>
          {hasActiveFilters && (
            <button type="button" className={styles.clearButton} onClick={clearFilters}>
              Clear filters
            </button>
          )}
        </div>
      ) : (
        <>
          <ul className={styles.grid}>
            {documents.map((doc) => (
              <DocumentTile
                key={doc.id}
                doc={doc}
                thumbnail={thumbnails[doc.id]}
                onOpen={() => void open(doc)}
                onDelete={() => void remove(doc.id)}
              />
            ))}
          </ul>

          {hasMore && (
            <button
              type="button"
              className={styles.loadMore}
              onClick={loadMore}
              disabled={isLoadingMore}>
              {isLoadingMore ? 'Loading…' : 'Load more'}
            </button>
          )}
        </>
      )}
    </section>
  );
}
