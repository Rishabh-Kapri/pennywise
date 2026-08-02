import { headlessApi, hasStoredSession, NotAuthenticatedError } from '../../utils/headlessApi';

/** Subset of shared/model.PipelineRun that the widget renders. */
export type PipelineRun = {
  id: string;
  status: 'running' | 'waiting_retry' | 'completed' | 'failed';
  currentStep: string;
  emailAccount?: string;
  emailsFetched: number;
  transactionsCreated: number;
  startedAt: string;
  updatedAt: string;
};

export type PipelineWidgetState =
  | { kind: 'loading' }
  | { kind: 'signedOut' }
  | { kind: 'error'; message: string }
  | { kind: 'retrying'; count: number }
  | { kind: 'ready'; parked: PipelineRun[]; latest: PipelineRun | null };

/** Cap on how many parked runs the widget will list or retry in one tap. */
const MAX_PARKED = 5;

export async function loadPipelineState(): Promise<PipelineWidgetState> {
  // Cheap pre-check so a signed-out device renders a sign-in prompt instead of
  // burning a request that is guaranteed to fail.
  if (!(await hasStoredSession())) return { kind: 'signedOut' };

  try {
    const [parked, recent] = await Promise.all([
      headlessApi.get<PipelineRun[]>(`pipeline/runs?status=waiting_retry&limit=${MAX_PARKED}`),
      headlessApi.get<PipelineRun[]>('pipeline/runs?limit=1')
    ]);

    return {
      kind: 'ready',
      parked: Array.isArray(parked) ? parked : [],
      latest: Array.isArray(recent) && recent.length > 0 ? recent[0] : null
    };
  } catch (error) {
    if (error instanceof NotAuthenticatedError) return { kind: 'signedOut' };
    return {
      kind: 'error',
      message: error instanceof Error ? error.message : 'Could not load pipeline'
    };
  }
}

/**
 * Signals a retry for each parked run. Runs sequentially rather than in
 * parallel: each retry signals a Temporal workflow, and a headless task is a
 * bad place to fan out writes.
 *
 * Returns how many were accepted; failures are swallowed so one bad run does
 * not strand the rest, and the caller re-reads state afterwards anyway.
 */
export async function retryParkedRuns(runs: PipelineRun[]): Promise<number> {
  let accepted = 0;
  for (const run of runs.slice(0, MAX_PARKED)) {
    try {
      await headlessApi.post(`pipeline/runs/${run.id}/retry`, {});
      accepted++;
    } catch (error) {
      console.log('[widget] retry failed for run', run.id, error);
    }
  }
  return accepted;
}
