import { useCallback, useEffect, useRef, useState } from 'react';
import { apiClient } from '@/utils';
import { openDocument, useImagePreviews } from './useImagePreviews';
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

/** Loads the budget-wide receipt library, one page at a time. */
export function useDocumentLibrary(filters: DocumentFilters): UseDocumentLibraryResult {
  const [documents, setDocuments] = useState<DocumentListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { previews: thumbnails, load, forget, clear } = useImagePreviews();
  // Guards against a slow first page landing after a newer filter's response
  // and overwriting it with stale rows.
  const requestIdRef = useRef(0);
  const [reloadToken, setReloadToken] = useState(0);

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
        void load(page, () => requestId === requestIdRef.current);
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
    [filters, load],
  );

  // fetchPage changes identity when the filters do, so this doubles as the
  // "filters changed, start over" effect; reloadToken forces the same reset
  // when the filters are unchanged but the caller asked for a refresh.
  useEffect(() => {
    clear();
    void fetchPage(0);
  }, [fetchPage, reloadToken, clear]);

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
      forget(documentId);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete document');
    }
  }, [forget]);

  const open = useCallback(async (doc: DocumentListItem) => {
    try {
      await openDocument(doc);
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
