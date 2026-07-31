import { useCallback, useEffect, useRef, useState } from 'react';
import { apiClient } from '@/utils';
import type { TransactionDocument } from '../types/transaction.types';

interface UseTransactionDocumentsResult {
  documents: TransactionDocument[];
  /** object URLs for image documents, keyed by document id */
  previewUrls: Record<string, string>;
  isLoading: boolean;
  isUploading: boolean;
  /** set while a multi-file batch is in flight */
  uploadProgress: { done: number; total: number } | null;
  error: string | null;
  upload: (files: File[]) => Promise<void>;
  /** merges page images server-side into one PDF document */
  uploadAsPdf: (files: File[]) => Promise<void>;
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
  const [uploadProgress, setUploadProgress] = useState<{ done: number; total: number } | null>(null);
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

  /**
   * Uploads one file per request (the API takes a single `file` part), so a
   * partial failure still keeps the pages that made it through.
   */
  const upload = useCallback(
    async (files: File[]) => {
      if (!transactionId || files.length === 0) return;
      setIsUploading(true);
      setError(null);
      setUploadProgress({ done: 0, total: files.length });
      const failed: string[] = [];

      for (const [index, file] of files.entries()) {
        try {
          const form = new FormData();
          form.append('file', file);
          const doc = await apiClient.postForm<TransactionDocument>(`transactions/${transactionId}/documents`, form);
          setDocuments((docs) => [...docs, doc]);
          void loadPreviews([doc]);
        } catch (err: unknown) {
          failed.push(file.name);
          console.error('Failed to upload', file.name, err);
        }
        setUploadProgress({ done: index + 1, total: files.length });
      }

      if (failed.length > 0) {
        setError(
          failed.length === files.length
            ? 'Failed to upload document'
            : `Uploaded ${files.length - failed.length} of ${files.length}; ${failed.length} failed`,
        );
      }
      setUploadProgress(null);
      setIsUploading(false);
    },
    [transactionId, loadPreviews],
  );

  const uploadAsPdf = useCallback(
    async (files: File[]) => {
      if (!transactionId || files.length === 0) return;
      setIsUploading(true);
      setError(null);
      try {
        const form = new FormData();
        // order matters — parts become pages in the order appended
        for (const file of files) form.append('pages', file);
        const doc = await apiClient.postForm<TransactionDocument>(
          `transactions/${transactionId}/documents/scan`,
          form,
        );
        setDocuments((docs) => [...docs, doc]);
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : 'Failed to build PDF');
      } finally {
        setIsUploading(false);
      }
    },
    [transactionId],
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

  return {
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
  };
}
