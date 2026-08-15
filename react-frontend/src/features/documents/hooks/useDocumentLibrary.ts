import { useCallback, useEffect, useRef, useState } from 'react';
import { apiClient } from '@/utils';
import type {
  DocumentFilters,
  DocumentLibraryResponse,
  DocumentListItem,
} from '../types/document.types';

const PAGE_SIZE = 40;

interface UseDocumentLibraryResult {
  documents: DocumentListItem[];
  /** object URLs for image documents, keyed by document id */
  thumbnails: Record<string, string>;
  total: number;
  hasMore: boolean;
  isLoading: boolean;
  isLoadingMore: boolean;
  error: string | null;
  loadMore: () => void;
  refresh: () => void;
  remove: (documentId: string) => Promise<void>;
  /** opens a document in a new tab using a short-lived blob URL */
  open: (doc: DocumentListItem) => Promise<void>;
}

function buildQuery(filters: DocumentFilters, offset: number): string {
  const params = new URLSearchParams();
  if (filters.search.trim()) params.set('search', filters.search.trim());
  if (filters.kind) params.set('type', filters.kind);
  if (filters.startDate) params.set('startDate', filters.startDate);
  if (filters.endDate) params.set('endDate', filters.endDate);
  params.set('limit', String(PAGE_SIZE));
  params.set('offset', String(offset));
  return params.toString();
}

/**
 * Loads the budget-wide receipt library, one page at a time.
 *
 * Thumbnails go through fetchBlobUrl because a plain <img src> can't carry the
 * Authorization header the content endpoint requires.
 */
export function useDocumentLibrary(filters: DocumentFilters): UseDocumentLibraryResult {
  const [documents, setDocuments] = useState<DocumentListItem[]>([]);
  const [thumbnails, setThumbnails] = useState<Record<string, string>>({});
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const urlsRef = useRef<Record<string, string>>({});
  // Guards against a slow first page landing after a newer filter's response
  // and overwriting it with stale rows.
  const requestIdRef = useRef(0);
  const [reloadToken, setReloadToken] = useState(0);

  const revokeAll = useCallback(() => {
    for (const url of Object.values(urlsRef.current)) URL.revokeObjectURL(url);
    urlsRef.current = {};
  }, []);

  const loadThumbnails = useCallback(async (docs: DocumentListItem[], requestId: number) => {
    for (const doc of docs) {
      if (!doc.mimeType.startsWith('image/') || urlsRef.current[doc.id]) continue;
      try {
        const url = await apiClient.fetchBlobUrl(`documents/${doc.id}/content`);
        // the filter moved on while this was in flight; the URL belongs to a
        // list that is no longer on screen
        if (requestId !== requestIdRef.current) {
          URL.revokeObjectURL(url);
          return;
        }
        urlsRef.current[doc.id] = url;
        setThumbnails({ ...urlsRef.current });
      } catch {
        // a missing thumbnail is non-fatal; the tile falls back to a file icon
      }
    }
  }, []);

  const fetchPage = useCallback(
    async (offset: number) => {
      const requestId = ++requestIdRef.current;
      if (offset === 0) {
        setIsLoading(true);
      } else {
        setIsLoadingMore(true);
      }
      setError(null);

      try {
        const res = await apiClient.get<DocumentLibraryResponse>(
          `documents?${buildQuery(filters, offset)}`,
        );
        if (requestId !== requestIdRef.current) return;

        const page = res?.data ?? [];
        setDocuments((prev) => (offset === 0 ? page : [...prev, ...page]));
        setTotal(res?.total ?? 0);
        setHasMore(Boolean(res?.hasMore));
        void loadThumbnails(page, requestId);
      } catch (err: unknown) {
        if (requestId !== requestIdRef.current) return;
        setError(err instanceof Error ? err.message : 'Failed to load documents');
      } finally {
        if (requestId === requestIdRef.current) {
          setIsLoading(false);
          setIsLoadingMore(false);
        }
      }
    },
    [filters, loadThumbnails],
  );

  // fetchPage changes identity when the filters do, so this doubles as the
  // "filters changed, start over" effect; reloadToken forces the same reset
  // when the filters are unchanged but the caller asked for a refresh.
  useEffect(() => {
    revokeAll();
    setThumbnails({});
    void fetchPage(0);
  }, [fetchPage, reloadToken, revokeAll]);

  useEffect(() => revokeAll, [revokeAll]);

  const loadMore = useCallback(() => {
    if (isLoading || isLoadingMore || !hasMore) return;
    void fetchPage(documents.length);
  }, [isLoading, isLoadingMore, hasMore, documents.length, fetchPage]);

  const refresh = useCallback(() => setReloadToken((token) => token + 1), []);

  const remove = useCallback(async (documentId: string) => {
    setError(null);
    try {
      await apiClient.delete(`documents/${documentId}`);
      setDocuments((docs) => docs.filter((doc) => doc.id !== documentId));
      setTotal((count) => Math.max(0, count - 1));
      const url = urlsRef.current[documentId];
      if (url) {
        URL.revokeObjectURL(url);
        delete urlsRef.current[documentId];
        setThumbnails({ ...urlsRef.current });
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete document');
    }
  }, []);

  const open = useCallback(async (doc: DocumentListItem) => {
    try {
      const url = await apiClient.fetchBlobUrl(`documents/${doc.id}/content`);
      window.open(url, '_blank', 'noopener');
      // give the new tab time to load before releasing the URL
      setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to open document');
    }
  }, []);

  return {
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
  };
}
