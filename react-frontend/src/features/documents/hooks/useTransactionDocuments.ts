import { useCallback, useEffect, useState } from 'react';
import { apiClient } from '@/utils';
import type { DocumentFile } from '../types/document.types';
import { openDocument, useImagePreviews } from './useImagePreviews';

interface UseTransactionDocumentsResult {
  documents: DocumentFile[];
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
  openDocument: (doc: DocumentFile) => Promise<void>;
}

/** Loads and manages the receipts/documents of a transaction. */
export function useTransactionDocuments(transactionId: string | undefined): UseTransactionDocumentsResult {
  const [documents, setDocuments] = useState<DocumentFile[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<{ done: number; total: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { previews: previewUrls, load: loadPreviews, forget, clear } = useImagePreviews();

  const refresh = useCallback(async () => {
    if (!transactionId) {
      setDocuments([]);
      return;
    }
    setIsLoading(true);
    setError(null);
    try {
      const docs = await apiClient.get<DocumentFile[]>(`transactions/${transactionId}/documents`);
      setDocuments(docs ?? []);
      void loadPreviews(docs ?? []);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load documents');
    } finally {
      setIsLoading(false);
    }
  }, [transactionId, loadPreviews]);

  useEffect(() => {
    clear();
    void refresh();
  }, [refresh, clear]);

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
          const doc = await apiClient.postForm<DocumentFile>(`transactions/${transactionId}/documents`, form);
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
        const doc = await apiClient.postForm<DocumentFile>(
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
      forget(documentId);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete document');
    }
  }, [forget]);

  const open = useCallback(async (doc: DocumentFile) => {
    try {
      await openDocument(doc);
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
    openDocument: open,
  };
}
