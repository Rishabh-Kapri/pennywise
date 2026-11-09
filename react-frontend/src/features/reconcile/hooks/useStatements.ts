import { useCallback, useEffect, useRef, useState } from 'react';
import { apiClient } from '@/utils';
import type {
  StatementCreateItem,
  StatementCreateResult,
  StatementImport,
  StatementReconciliation,
  StatementUploadResponse,
} from '../types/reconcile.types';

// Extraction is a background job on the API (several LLM round-trips); while
// anything is processing, the list and the open statement are polled.
const POLL_MS = 3000;

export interface UploadOutcome {
  fileName: string;
  ok: boolean;
  message?: string;
  importId?: string;
}

export function useStatementList() {
  const [imports, setImports] = useState<StatementImport[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const list = await apiClient.get<StatementImport[]>('statements');
      setImports(list ?? []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load statements');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const anyProcessing = imports.some((imp) => imp.status === 'processing');
  useEffect(() => {
    if (!anyProcessing) return;
    const timer = window.setInterval(() => void refresh(), POLL_MS);
    return () => window.clearInterval(timer);
  }, [anyProcessing, refresh]);

  /** Uploads one file at a time: the API queues extraction serially anyway. */
  const upload = useCallback(
    async (files: File[], accountId: string, password: string): Promise<UploadOutcome[]> => {
      const outcomes: UploadOutcome[] = [];
      for (const file of files) {
        const form = new FormData();
        form.append('file', file);
        if (accountId) form.append('accountId', accountId);
        if (password) form.append('password', password);
        try {
          const res = await apiClient.postForm<StatementUploadResponse>('statements', form);
          outcomes.push({
            fileName: file.name,
            ok: true,
            importId: res.import.id,
            message: res.duplicate ? 'Already uploaded — opened the existing import' : undefined,
          });
        } catch (err) {
          outcomes.push({
            fileName: file.name,
            ok: false,
            message: err instanceof Error ? err.message : 'Upload failed',
          });
        }
      }
      await refresh();
      return outcomes;
    },
    [refresh],
  );

  const remove = useCallback(
    async (id: string) => {
      await apiClient.delete(`statements/${id}`);
      await refresh();
    },
    [refresh],
  );

  const retry = useCallback(
    async (id: string) => {
      await apiClient.post(`statements/${id}/retry`, {});
      await refresh();
    },
    [refresh],
  );

  return { imports, isLoading, error, refresh, upload, remove, retry };
}

export function useStatementReconciliation(id: string | null) {
  const [data, setData] = useState<StatementReconciliation | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // a slow response for a previously open statement must not overwrite this one
  const currentId = useRef(id);
  currentId.current = id;

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const res = await apiClient.get<StatementReconciliation>(`statements/${id}`);
      if (currentId.current !== id) return;
      setData(res);
      setError(null);
    } catch (err) {
      if (currentId.current !== id) return;
      setError(err instanceof Error ? err.message : 'Failed to load statement');
    } finally {
      if (currentId.current === id) setIsLoading(false);
    }
  }, [id]);

  useEffect(() => {
    setData(null);
    if (!id) return;
    setIsLoading(true);
    void load();
  }, [id, load]);

  const processing = data?.import.status === 'processing';
  useEffect(() => {
    if (!processing) return;
    const timer = window.setInterval(() => void load(), POLL_MS);
    return () => window.clearInterval(timer);
  }, [processing, load]);

  const setSectionAccount = useCallback(
    async (section: number, accountId: string | null) => {
      if (!id) return;
      const res = await apiClient.patch<StatementReconciliation, { accountId: string | null }>(
        `statements/${id}/sections/${section}`,
        { accountId },
      );
      setData(res);
    },
    [id],
  );

  const createMissing = useCallback(
    async (items: StatementCreateItem[]) => {
      if (!id) throw new Error('No statement selected');
      const res = await apiClient.post<StatementCreateResult, { items: StatementCreateItem[] }>(
        `statements/${id}/transactions`,
        { items },
      );
      await load();
      return res;
    },
    [id, load],
  );

  return { data, isLoading, error, reload: load, setSectionAccount, createMissing };
}
