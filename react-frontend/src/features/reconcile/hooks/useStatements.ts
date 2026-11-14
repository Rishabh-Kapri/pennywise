import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useAppSelector } from '@/app/hooks';
import type { JobUpdate } from '@/features/jobs/types/job.types';
import { apiClient } from '@/utils';
import type {
  StatementCreateItem,
  StatementCreateResult,
  StatementImport,
  StatementReconciliation,
  StatementUploadResponse,
} from '../types/reconcile.types';

// A statement is read by a job in cipher (statement.extract), then saved onto
// its import by the API (statement.save). Progress arrives as job updates on
// the websocket, matched to an import by its jobId. When a statement job ends,
// the screens reload. Updates can be dropped, so a slow poll backs them up.
const FALLBACK_POLL_MS = 20000;

/** The import with its job's latest progress, while it is processing. */
export function withLiveProgress(imp: StatementImport, latest: Record<string, JobUpdate>): StatementImport {
  if (imp.status !== 'processing' || !imp.jobId) return imp;
  const progress = latest[imp.jobId]?.progress;
  if (!progress?.message) return imp;
  return {
    ...imp,
    stage: progress.message,
    pagesProcessed: progress.current ?? 0,
    pageCount: progress.total ?? 0,
  };
}

/** "Reading pages 3/7", or what the job is doing when it has no count yet. */
export function statementProgressLabel(imp: StatementImport): string {
  const stage = imp.stage ? imp.stage.charAt(0).toUpperCase() + imp.stage.slice(1) : 'Waiting to start';
  return imp.pageCount ? `${stage} ${imp.pagesProcessed}/${imp.pageCount}…` : `${stage}…`;
}

/** Reloads when a statement job ends, and every so often while processing. */
function useStatementRefresh(processing: boolean, reload: () => unknown) {
  const statementVersion = useAppSelector((state) => state.jobs.statementVersion);
  const seen = useRef(statementVersion);
  useEffect(() => {
    if (seen.current === statementVersion) return;
    seen.current = statementVersion;
    if (processing) void reload();
  }, [statementVersion, processing, reload]);

  useEffect(() => {
    if (!processing) return;
    const timer = window.setInterval(() => void reload(), FALLBACK_POLL_MS);
    return () => window.clearInterval(timer);
  }, [processing, reload]);
}

/**
 * Opens the uploaded PDF in a new tab. The file needs auth headers, so it is
 * fetched as a blob first. The tab is opened before the fetch, while the click
 * still counts as a user action, or the popup blocker would stop it.
 */
export async function openStatementPdf(id: string): Promise<void> {
  const tab = window.open('', '_blank');
  try {
    const url = await apiClient.fetchBlobUrl(`statements/${id}/file`);
    if (tab) tab.location.href = url;
    else window.location.href = url;
    // the tab has loaded it by then; the URL only holds the blob in memory
    window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
  } catch (err) {
    tab?.close();
    throw err;
  }
}

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

  const latest = useAppSelector((state) => state.jobs.latest);
  const liveImports = useMemo(() => imports.map((imp) => withLiveProgress(imp, latest)), [imports, latest]);
  const anyProcessing = imports.some((imp) => imp.status === 'processing');
  useStatementRefresh(anyProcessing, refresh);

  /** Uploads one file at a time, so a failure says which file it was. */
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

  return { imports: liveImports, isLoading, error, refresh, upload, remove, retry };
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
  useStatementRefresh(processing, load);
  const latest = useAppSelector((state) => state.jobs.latest);
  const liveData = useMemo(
    () => (data ? { ...data, import: withLiveProgress(data.import, latest) } : null),
    [data, latest],
  );

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

  return { data: liveData, isLoading, error, reload: load, setSectionAccount, createMissing };
}
