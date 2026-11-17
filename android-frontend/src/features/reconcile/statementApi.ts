import { useCallback, useEffect, useState } from 'react';
import { apiClient } from '../../utils/api';
import type {
  StatementCreateItem,
  StatementCreateResult,
  StatementDifferenceRequest,
  StatementImport,
  StatementReconciliation
} from './types';

// A statement is read by a job in cipher. The web follows that job over the
// websocket; this app has no job updates, so screens poll while one is read.
const PROCESSING_POLL_MS = 5000;

/** "Reading pages 3/7", or what the job is doing when it has no count yet. */
export function statementProgressLabel(imp: StatementImport): string {
  const stage = imp.stage ? imp.stage.charAt(0).toUpperCase() + imp.stage.slice(1) : 'Waiting to start';
  return imp.pageCount ? `${stage} ${imp.pagesProcessed}/${imp.pageCount}…` : `${stage}…`;
}

function usePollWhile(active: boolean, reload: () => unknown) {
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(() => void reload(), PROCESSING_POLL_MS);
    return () => clearInterval(timer);
  }, [active, reload]);
}

/** A PDF picked on the phone, as a multipart file part. */
export interface PickedPdf {
  uri: string;
  name: string;
  type: string;
}

export interface UploadOutcome {
  fileName: string;
  ok: boolean;
  message?: string;
}

export function useStatementList() {
  const [imports, setImports] = useState<StatementImport[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setImports((await apiClient.get<StatementImport[]>('statements')) ?? []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load statements');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);
  usePollWhile(
    imports.some((imp) => imp.status === 'processing'),
    load
  );

  const refresh = useCallback(async () => {
    setIsRefreshing(true);
    await load();
    setIsRefreshing(false);
  }, [load]);

  /** One file at a time, so a failure says which file it was. */
  const upload = useCallback(
    async (files: PickedPdf[], accountId: string, password: string): Promise<UploadOutcome[]> => {
      const outcomes: UploadOutcome[] = [];
      for (const file of files) {
        const form = new FormData();
        // React Native FormData file part
        form.append('file', file as unknown as Blob);
        if (accountId) form.append('accountId', accountId);
        if (password) form.append('password', password);
        try {
          const res = await apiClient.postForm<{ duplicate: boolean }>('statements', form);
          outcomes.push({
            fileName: file.name,
            ok: true,
            message: res.duplicate ? 'Already uploaded' : undefined
          });
        } catch (err) {
          outcomes.push({
            fileName: file.name,
            ok: false,
            message: err instanceof Error ? err.message : 'Upload failed'
          });
        }
      }
      await load();
      return outcomes;
    },
    [load]
  );

  const remove = useCallback(
    async (id: string) => {
      await apiClient.delete(`statements/${id}`);
      await load();
    },
    [load]
  );

  const retry = useCallback(
    async (id: string) => {
      await apiClient.post(`statements/${id}/retry`, {});
      await load();
    },
    [load]
  );

  return {
    imports,
    isLoading,
    isRefreshing,
    error,
    refresh,
    upload,
    remove,
    retry
  };
}

export function useStatementReconciliation(id: string) {
  const [data, setData] = useState<StatementReconciliation | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setData(await apiClient.get<StatementReconciliation>(`statements/${id}`));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load the statement');
    } finally {
      setIsLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);
  usePollWhile(data?.import.status === 'processing', load);

  const setSectionAccount = useCallback(
    async (section: number, accountId: string | null) => {
      setData(await apiClient.patch<StatementReconciliation>(`statements/${id}/sections/${section}`, { accountId }));
    },
    [id]
  );

  const createMissing = useCallback(
    async (items: StatementCreateItem[]) => {
      const res = await apiClient.post<StatementCreateResult>(`statements/${id}/transactions`, { items });
      await load();
      return res;
    },
    [id, load]
  );

  const linkRow = useCallback(
    async (rowId: string, transactionIds: string[]) => {
      setData(
        await apiClient.put<StatementReconciliation>(`statements/${id}/rows/${encodeURIComponent(rowId)}/link`, {
          transactionIds
        })
      );
    },
    [id]
  );

  const unlinkRow = useCallback(
    async (rowId: string) => {
      setData(await apiClient.delete<StatementReconciliation>(`statements/${id}/rows/${encodeURIComponent(rowId)}/link`));
    },
    [id]
  );

  // say whether a row is part of an EMI already in the budget; null goes back to the row's text
  const setRowEMI = useCallback(
    async (rowId: string, emi: boolean | null) => {
      setData(await apiClient.put<StatementReconciliation>(`statements/${id}/rows/${encodeURIComponent(rowId)}/emi`, { emi }));
    },
    [id]
  );

  const addDifference = useCallback(
    async (rowId: string, body: StatementDifferenceRequest) => {
      setData(
        await apiClient.post<StatementReconciliation>(`statements/${id}/rows/${encodeURIComponent(rowId)}/difference`, body)
      );
    },
    [id]
  );

  return {
    data,
    isLoading,
    error,
    reload: load,
    setSectionAccount,
    createMissing,
    linkRow,
    unlinkRow,
    setRowEMI,
    addDifference
  };
}
