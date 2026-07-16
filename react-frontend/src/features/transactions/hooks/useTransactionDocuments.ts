import { useCallback, useEffect, useRef, useState } from 'react';
import { apiClient } from '@/utils';
import type { TransactionDocument } from '../types/transaction.types';

interface UseTransactionDocumentsResult {
  documents: TransactionDocument[];
  /** object URLs for image documents, keyed by document id */
  previewUrls: Record<string, string>;
  isLoading: boolean;
  isUploading: boolean;
  error: string | null;
  upload: (file: File) => Promise<void>;
  remove: (documentId: string) => Promise<void>;
  /** fetch a fresh blob URL for opening/downloading any document */
  openDocument: (doc: TransactionDocument) => Promise<void>;
}

/**
 * Loads and manages the receipts/documents of a transaction. Image previews go
 * through fetchBlobUrl because plain <img src> can't carry the auth header.
 */
export function useTransactionDocuments(transactionId: string | undefined): UseTransactionDocumentsResult {
  const [documents, setDocuments] = useState<TransactionDocument[]>([]);
  const [previewUrls, setPreviewUrls] = useState<Record<string, string>>({});
  const [isLoading, setIsLoading] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const urlsRef = useRef<Record<string, string>>({});

  const revokeAll = () => {
    for (const url of Object.values(urlsRef.current)) {
      URL.revokeObjectURL(url);
    }
    urlsRef.current = {};
  };

  const loadPreviews = useCallback(async (docs: TransactionDocument[]) => {
    for (const doc of docs) {
      if (!doc.mimeType.startsWith('image/') || urlsRef.current[doc.id]) continue;
      try {
        const url = await apiClient.fetchBlobUrl(`documents/${doc.id}/content`);
        urlsRef.current[doc.id] = url;
        setPreviewUrls({ ...urlsRef.current });
      } catch {
        // preview failures are non-fatal; the row still shows a file chip
      }
    }
  }, []);

  const refresh = useCallback(async () => {
    if (!transactionId) {
      setDocuments([]);
      return;
    }
    setIsLoading(true);
    setError(null);
    try {
      const docs = await apiClient.get<TransactionDocument[]>(`transactions/${transactionId}/documents`);
      setDocuments(docs ?? []);
      void loadPreviews(docs ?? []);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load documents');
    } finally {
      setIsLoading(false);
    }
  }, [transactionId, loadPreviews]);

  useEffect(() => {
    revokeAll();
    setPreviewUrls({});
    void refresh();
    return revokeAll;
  }, [refresh]);

  const upload = useCallback(
    async (file: File) => {
      if (!transactionId) return;
      setIsUploading(true);
      setError(null);
      try {
        const form = new FormData();
        form.append('file', file);
        const doc = await apiClient.postForm<TransactionDocument>(`transactions/${transactionId}/documents`, form);
        setDocuments((docs) => [...docs, doc]);
        void loadPreviews([doc]);
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : 'Failed to upload document');
      } finally {
        setIsUploading(false);
      }
    },
    [transactionId, loadPreviews],
  );

  const remove = useCallback(async (documentId: string) => {
    setError(null);
    try {
      await apiClient.delete(`documents/${documentId}`);
      setDocuments((docs) => docs.filter((doc) => doc.id !== documentId));
      const url = urlsRef.current[documentId];
      if (url) {
        URL.revokeObjectURL(url);
        delete urlsRef.current[documentId];
        setPreviewUrls({ ...urlsRef.current });
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete document');
    }
  }, []);

  const openDocument = useCallback(async (doc: TransactionDocument) => {
    try {
      const url = await apiClient.fetchBlobUrl(`documents/${doc.id}/content`);
      window.open(url, '_blank', 'noopener');
      // give the new tab time to load before revoking
      setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to open document');
    }
  }, []);

  return { documents, previewUrls, isLoading, isUploading, error, upload, remove, openDocument };
}
