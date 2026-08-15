import { useCallback, useEffect, useRef, useState } from 'react';
import * as FileSystem from 'expo-file-system/legacy';
import { apiClient } from '../../../utils/api';
import {
  buildDocumentQuery,
  type DocumentFilters,
  type DocumentLibraryResponse,
  type DocumentListItem
} from '../types';

const PAGE_SIZE = 30;

interface UseDocumentLibraryResult {
  documents: DocumentListItem[];
  /** cached local file paths for image documents, keyed by document id */
  thumbnails: Record<string, string>;
  total: number;
  hasMore: boolean;
  isLoading: boolean;
  isLoadingMore: boolean;
  isRefreshing: boolean;
  error: string | null;
  loadMore: () => void;
  refresh: () => void;
  remove: (documentId: string) => Promise<void>;
}

/**
 * Loads the budget-wide receipt library a page at a time.
 *
 * Thumbnails are downloaded into the cache directory with auth headers, since
 * a plain <Image source={{uri}}> can't send Authorization. The same cache is
 * shared with TransactionAttachments (identical `receipt-<id>` naming), so a
 * receipt already viewed on its transaction renders here without a refetch.
 */
export function useDocumentLibrary(filters: DocumentFilters): UseDocumentLibraryResult {
  const [documents, setDocuments] = useState<DocumentListItem[]>([]);
  const [thumbnails, setThumbnails] = useState<Record<string, string>>({});
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Guards a slow response for an older filter from overwriting a newer one.
  const requestIdRef = useRef(0);
  const [reloadToken, setReloadToken] = useState(0);

  const loadThumbnails = useCallback(async (docs: DocumentListItem[], requestId: number) => {
    for (const doc of docs) {
      if (!doc.mimeType.startsWith('image/')) continue;
      try {
        const { url, headers } = apiClient.getAuthorizedRequest(`documents/${doc.id}/content`);
        const ext = doc.fileName.includes('.') ? doc.fileName.slice(doc.fileName.lastIndexOf('.')) : '.jpg';
        const target = `${FileSystem.cacheDirectory}receipt-${doc.id}${ext}`;

        const info = await FileSystem.getInfoAsync(target);
        if (!info.exists) {
          const result = await FileSystem.downloadAsync(url, target, { headers });
          if (result.status !== 200) continue;
        }
        // the list moved on while this was downloading
        if (requestId !== requestIdRef.current) return;
        setThumbnails((prev) => ({ ...prev, [doc.id]: target }));
      } catch {
        // a missing thumbnail is non-fatal; the tile falls back to a file icon
      }
    }
  }, []);

  const fetchPage = useCallback(
    async (offset: number) => {
      const requestId = ++requestIdRef.current;
      if (offset > 0) setIsLoadingMore(true);
      setError(null);

      try {
        const res = await apiClient.get<DocumentLibraryResponse>(
          `documents?${buildDocumentQuery(filters, offset, PAGE_SIZE)}`
        );
        if (requestId !== requestIdRef.current) return;

        const page = res?.data ?? [];
        setDocuments((prev) => (offset === 0 ? page : [...prev, ...page]));
        setTotal(res?.total ?? 0);
        setHasMore(Boolean(res?.hasMore));
        void loadThumbnails(page, requestId);
      } catch (err) {
        if (requestId !== requestIdRef.current) return;
        setError(err instanceof Error ? err.message : 'Failed to load documents');
      } finally {
        if (requestId === requestIdRef.current) {
          setIsLoading(false);
          setIsLoadingMore(false);
          setIsRefreshing(false);
        }
      }
    },
    [filters, loadThumbnails]
  );

  // fetchPage changes identity with the filters, so this doubles as the
  // "filters changed, start over" effect; reloadToken forces the same reset
  // for a pull-to-refresh where the filters are unchanged.
  useEffect(() => {
    void fetchPage(0);
  }, [fetchPage, reloadToken]);

  const loadMore = useCallback(() => {
    if (isLoading || isLoadingMore || !hasMore) return;
    void fetchPage(documents.length);
  }, [isLoading, isLoadingMore, hasMore, documents.length, fetchPage]);

  const refresh = useCallback(() => {
    setIsRefreshing(true);
    setReloadToken((token) => token + 1);
  }, []);

  const remove = useCallback(async (documentId: string) => {
    setError(null);
    try {
      await apiClient.delete(`documents/${documentId}`);
      setDocuments((docs) => docs.filter((doc) => doc.id !== documentId));
      setTotal((count) => Math.max(0, count - 1));
      setThumbnails((prev) => {
        if (!prev[documentId]) return prev;
        const next = { ...prev };
        delete next[documentId];
        return next;
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete document');
    }
  }, []);

  return {
    documents,
    thumbnails,
    total,
    hasMore,
    isLoading,
    isLoadingMore,
    isRefreshing,
    error,
    loadMore,
    refresh,
    remove
  };
}
